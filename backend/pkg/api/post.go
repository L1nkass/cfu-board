package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cfu-board/pkg/database"
	"cfu-board/pkg/repository"
	"cfu-board/pkg/storage"
)

const (
	maxUploadSize  = 500 << 20 // 500 MB — жёсткий лимит всего запроса (защита диска/памяти)
	maxFormMemory  = 32 << 20  // 32 MB в оперативной памяти, остальное — во временные файлы
	sniffBufferLen = 512       // сколько байт читаем для определения реального типа файла
	dbOpTimeout    = 5 * time.Second
)

// mediaSpec описывает допустимый тип медиа: список расширений и функция
// проверки «магических байтов» (реального содержимого) файла.
type mediaSpec struct {
	exts   []string
	label  string // человекочитаемый список форматов для сообщения об ошибке
	verify func(head []byte) bool
}

var (
	imageSpec = mediaSpec{
		exts:  []string{".jpg", ".jpeg", ".png", ".webp"},
		label: "JPG, PNG, WEBP",
		verify: func(h []byte) bool {
			return bytes.HasPrefix(h, []byte{0xFF, 0xD8, 0xFF}) || // JPEG
				bytes.HasPrefix(h, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) || // PNG
				(len(h) >= 12 && bytes.Equal(h[0:4], []byte("RIFF")) && bytes.Equal(h[8:12], []byte("WEBP"))) // WEBP
		},
	}
	videoSpec = mediaSpec{
		exts:  []string{".mp4", ".mov", ".avi"},
		label: "MP4, MOV, AVI",
		verify: func(h []byte) bool {
			isMP4Family := len(h) >= 8 && bytes.Equal(h[4:8], []byte("ftyp")) // MP4 и MOV (QuickTime)
			isAVI := len(h) >= 12 && bytes.Equal(h[0:4], []byte("RIFF")) && bytes.Equal(h[8:12], []byte("AVI "))
			return isMP4Family || isAVI
		},
	}
)

// contentTypeForExt возвращает корректный MIME-тип по расширению файла,
// чтобы хранилище отдавало файл с правильным Content-Type.
func contentTypeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	default:
		return "application/octet-stream"
	}
}

// CreatePostMultipartHandler обрабатывает загрузку текста и медиафайлов
// (POST /posts/create). Медиа сохраняется через абстракцию storage.Storage
// (локальный диск или MinIO). Если запись в БД не удалась, уже загруженные
// объекты удаляются из хранилища, чтобы не оставлять мусор.
func CreatePostMultipartHandler(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		// 1. ОГРАНИЧЕНИЕ РАЗМЕРА ВСЕГО ЗАПРОСА (Max 500 MB под тяжёлые видео).
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

		// 2. Парсим multipart-форму. Часть держим в памяти, крупные файлы
		// уходят во временные файлы на диске.
		if err := r.ParseMultipartForm(maxFormMemory); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Не удалось обработать форму. Возможно, превышен лимит 500MB")
			return
		}
		defer func() { _ = r.MultipartForm.RemoveAll() }()

		// 3. Достаём контекст авторизации из нашего Middleware.
		authorID, ok := r.Context().Value(UserIDKey).(int)
		if !ok {
			RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		// 3.1 Создавать посты могут только пользователи с подтверждённым email.
		// Берём АКТУАЛЬНЫЙ статус из БД, а не из JWT: пользователь мог
		// верифицироваться уже после получения токена, а прежний access-токен
		// «не знает» об этом.
		userRepo := repository.NewUserRepository(database.GlobalPool)
		checkCtx, checkCancel := context.WithTimeout(r.Context(), dbOpTimeout)
		defer checkCancel()
		author, err := userRepo.GetByID(checkCtx, authorID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusUnauthorized, "Пользователь не найден")
				return
			}
			slog.Error("не удалось загрузить автора поста из БД", "author_id", authorID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при проверке аккаунта")
			return
		}
		if !author.IsVerified {
			RespondWithError(w, http.StatusForbidden, "Публиковать посты могут только пользователи с подтверждённым email")
			return
		}

		// 4. Читаем текстовые поля из формы.
		title := strings.TrimSpace(r.FormValue("title"))
		content := strings.TrimSpace(r.FormValue("content"))
		if title == "" || content == "" {
			RespondWithError(w, http.StatusBadRequest, "Заголовок и текст публикации обязательны")
			return
		}

		// Сохраняем URL загруженных объектов, чтобы удалить их из хранилища,
		// если запись в БД не удастся (компенсирующее удаление).
		var uploaded []string
		cleanup := func() {
			ctx, cancel := context.WithTimeout(context.Background(), dbOpTimeout)
			defer cancel()
			for _, u := range uploaded {
				if err := store.Delete(ctx, u); err != nil {
					slog.Warn("не удалось удалить объект после сбоя записи в БД", "url", u, "error", err)
				}
			}
		}

		// 5. ПОТОКОВАЯ ОБРАБОТКА КАРТИНКИ.
		imageURL, err := saveMediaFile(r, store, "image", authorID, imageSpec)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		if imageURL != "" {
			uploaded = append(uploaded, imageURL)
		}

		// 6. ПОТОКОВАЯ ОБРАБОТКА ВИДЕО.
		videoURL, err := saveMediaFile(r, store, "video", authorID, videoSpec)
		if err != nil {
			cleanup() // удаляем уже сохранённую картинку, раз видео невалидно
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		if videoURL != "" {
			uploaded = append(uploaded, videoURL)
		}

		// 7. Записываем ссылки в PostgreSQL через репозиторий.
		dbCtx, cancel := context.WithTimeout(r.Context(), dbOpTimeout)
		defer cancel()
		if err := userRepo.CreatePostWithMedia(dbCtx, authorID, title, content, imageURL, videoURL); err != nil {
			slog.Error("ошибка сохранения медиа-поста в БД", "error", err)
			cleanup() // чистим осиротевшие объекты, раз запись в БД не удалась
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сохранения в базу данных")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"status":"success","message":"Публикация с медиа успешно размещена!"}`)
	}
}

// saveMediaFile загружает переданный в форме файл в хранилище, предварительно
// проверив расширение и реальное содержимое («магические байты»). Если файл не
// приложен, возвращает пустую строку без ошибки (медиа необязательно). Ключ
// объекта формируется как "posts/<authorID>_<timestamp><ext>".
func saveMediaFile(r *http.Request, store storage.Storage, field string, authorID int, spec mediaSpec) (string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		// Поле отсутствует — это не ошибка, медиа необязательно.
		if errors.Is(err, http.ErrMissingFile) {
			return "", nil
		}
		return "", fmt.Errorf("не удалось получить файл '%s': %w", field, err)
	}
	defer file.Close()

	ext := strings.ToLower(path.Ext(header.Filename))
	if !slices.Contains(spec.exts, ext) {
		return "", fmt.Errorf("для поля '%s' разрешены только форматы %s", field, spec.label)
	}

	// Читаем начало файла для проверки реального содержимого.
	head := make([]byte, sniffBufferLen)
	n, err := io.ReadFull(file, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", fmt.Errorf("не удалось прочитать файл '%s': %w", field, err)
	}
	head = head[:n]
	if !spec.verify(head) {
		return "", fmt.Errorf("содержимое файла '%s' не соответствует заявленному расширению %s", field, ext)
	}

	// Склеиваем уже прочитанное начало с остатком потока и загружаем в хранилище.
	reader := io.MultiReader(bytes.NewReader(head), file)
	key := fmt.Sprintf("posts/%d_%d%s", authorID, time.Now().UnixNano(), ext)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	url, err := store.Save(ctx, key, reader, header.Size, contentTypeForExt(ext))
	if err != nil {
		return "", fmt.Errorf("не удалось сохранить файл '%s': %w", field, err)
	}
	return url, nil
}

// postFeedLimit ограничивает пагинацию ленты: по умолчанию 20, потолок 100.
const (
	defaultPostsLimit = 20
	maxPostsLimit     = 100
)

// ListPostsHandler обрабатывает ленту публикаций (GET /posts?limit=&offset=).
// Публичный эндпоинт: лента видна всем, при валидном токене поле liked_by_me
// персонализируется (OptionalAuthMiddleware). Сортировка — новые сверху
// (created_at DESC, id DESC). Ответ — массив постов; пустая лента — [] а не null.
func ListPostsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		limit, err := parseQueryInt(r, "limit", defaultPostsLimit)
		if err != nil || limit < 1 || limit > maxPostsLimit {
			RespondWithError(w, http.StatusBadRequest, "Параметр limit: целое число от 1 до 100")
			return
		}
		offset, err := parseQueryInt(r, "offset", 0)
		if err != nil || offset < 0 {
			RespondWithError(w, http.StatusBadRequest, "Параметр offset: целое неотрицательное число")
			return
		}

		// Для анонимов UserID в контексте нет — ListPostsWithLikes трактует 0
		// как «никого не смотрим», liked_by_me=false.
		viewerID, _ := r.Context().Value(UserIDKey).(int)

		userRepo := repository.NewUserRepository(database.GlobalPool)
		dbCtx, cancel := context.WithTimeout(r.Context(), dbOpTimeout)
		defer cancel()

		posts, err := userRepo.ListPostsWithLikes(dbCtx, viewerID, limit, offset)
		if err != nil {
			slog.Error("не удалось загрузить ленту постов", "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке публикаций")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(posts)
	}
}

// DeletePostHandler обрабатывает удаление публикации по её ID (DELETE /posts/{id}).
// Удалить пост может только его автор: ID пользователя берётся из JWT-контекста и
// сверяется с автором поста и в приложении (для понятного 403), и в самом
// SQL-запросе (атомарная защита от гонок). После удаления записи из хранилища
// удаляются связанные медиафайлы, чтобы не оставлять мусор.
func DeletePostHandler(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			RespondWithError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}

		postID, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
		if err != nil || postID <= 0 {
			RespondWithError(w, http.StatusBadRequest, "Некорректный идентификатор публикации")
			return
		}

		authorID, ok := r.Context().Value(UserIDKey).(int)
		if !ok {
			RespondWithError(w, http.StatusUnauthorized, "Требуется авторизация")
			return
		}

		userRepo := repository.NewUserRepository(database.GlobalPool)
		dbCtx, cancel := context.WithTimeout(r.Context(), dbOpTimeout)
		defer cancel()

		// Сначала грузим пост: чтобы отличить «не существует» (404) от
		// «чужой» (403) и узнать URL медиа для последующей очистки.
		post, err := userRepo.GetPostByID(dbCtx, postID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
				return
			}
			slog.Error("не удалось загрузить пост для удаления", "post_id", postID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке публикации")
			return
		}

		if post.AuthorID != authorID {
			RespondWithError(w, http.StatusForbidden, "Удалить публикацию может только её автор")
			return
		}

		// Удаляем запись, ещё раз проверяя авторство прямо в SQL (защита от гонок).
		deleted, err := userRepo.DeletePostByID(dbCtx, postID, authorID)
		if err != nil {
			slog.Error("не удалось удалить пост из БД", "post_id", postID, "error", err)
			RespondWithError(w, http.StatusInternalServerError, "Ошибка удаления публикации")
			return
		}
		if !deleted {
			// Между чтением и удалением пост мог исчезнуть.
			RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
			return
		}

		// Подчищаем медиафайлы из хранилища (best-effort: ошибки удаления не
		// критичны, ведь запись в БД уже удалена — осиротевший объект не
		// должен ронять ответ).
		delCtx, delCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer delCancel()
		for _, u := range []string{post.ImageURL, post.VideoURL} {
			if u == "" {
				continue
			}
			if err := store.Delete(delCtx, u); err != nil {
				slog.Warn("не удалось удалить медиафайл поста", "post_id", postID, "url", u, "error", err)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"success","message":"Публикация успешно удалена"}`)
	}
}
