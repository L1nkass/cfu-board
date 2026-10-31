package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"

	"cfu-board/internal/models"
	"cfu-board/internal/modules/post/repository"
	"cfu-board/internal/platform/middleware"
)

// LoadVisiblePost загружает пост и применяет то же правило видимости, что
// GET /posts/{id}: pending/rejected видит только автор. Чужим отдаём 404,
// чтобы не подтверждать существование скрытой публикации (лайки, комментарии).
func LoadVisiblePost(
	w http.ResponseWriter,
	repo *repository.Repository,
	ctx context.Context,
	postID, viewerID int,
) (*models.Post, bool) {
	post, err := repo.GetPostByID(ctx, postID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			middleware.RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
			return nil, false
		}
		slog.Error("не удалось загрузить публикацию", "post_id", postID, "error", err)
		middleware.RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке публикации")
		return nil, false
	}
	if !post.IsVisibleToViewer(viewerID) {
		middleware.RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
		return nil, false
	}
	return post, true
}
