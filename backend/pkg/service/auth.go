package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"cfu-board/pkg/config"
	"cfu-board/pkg/models"
	"cfu-board/pkg/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

// Кастомные ошибки для четкого дебаггинга и безопасности системы
var (
	ErrInvalidInput     = errors.New("все поля обязательны к заполнению")
	ErrUsernameLength   = errors.New("имя пользователя должно быть от 3 до 30 символов")
	ErrUsernameChars    = errors.New("имя пользователя может содержать только латинские буквы, цифры, знаки '_' и '-'")
	ErrEmailTooLong     = errors.New("email слишком длинный (максимум 254 символа)")
	ErrEmailInvalid     = errors.New("некорректный формат email адреса")
	ErrPasswordLength   = errors.New("пароль должен быть от 8 до 72 символов")
	ErrUsernameTaken    = errors.New("это имя пользователя уже занято")
	ErrEmailRegistered  = errors.New("этот email уже зарегистрирован")
	ErrAuthFailed       = errors.New("неверный логин или пароль")
	ErrTokenInvalid     = errors.New("ссылка для верификации недействительна или её срок исчек")
	ErrTokenExpired     = errors.New("срок действия токена истёк")
	ErrTokenAlreadyUsed = errors.New("токен уже был использован")
	ErrUserNotFound     = errors.New("пользователь не найден")
	ErrAlreadyVerified  = errors.New("аккаунт уже был успешно подтвержден ранее")
	ErrInternal         = errors.New("внутренняя ошибка сервера")
)

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// UserClaims описывает структуру полезной нагрузки внутри JWT
type UserClaims struct {
	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	IsAdmin    bool   `json:"is_admin"`
	IsVerified bool   `json:"is_verified"`
	jwt.RegisteredClaims
}

// LoginResult — структура для передачи результатов авторизации в хендлер.
// Содержит пару токенов: короткоживущий access и долгоживущий refresh.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	UserID       int
	Username     string
	Email        string
	IsVerified   bool
}

type AuthService struct {
	userRepo       *repository.UserRepository
	refreshRepo    *repository.RefreshTokenRepository
	verifRepo      *repository.EmailVerificationRepository
	mailService    MailService
	jwtSecret      []byte
	accessTTL      time.Duration
	refreshTTL     time.Duration
	verifTTL       time.Duration
	resendCooldown time.Duration
	maxVerifPerHr  int
	baseURL        string
}

// NewAuthService — конструктор сервиса авторизации. Зависимости (репозиторий
// пользователей, репозиторий refresh-токенов, репозиторий токенов верификации,
// почтовый сервис, конфигурация) передаются извне (проброс зависимостей).
func NewAuthService(
	repo *repository.UserRepository,
	refreshRepo *repository.RefreshTokenRepository,
	verifRepo *repository.EmailVerificationRepository,
	mail MailService,
	cfg *config.Config,
) *AuthService {
	return &AuthService{
		userRepo:       repo,
		refreshRepo:    refreshRepo,
		verifRepo:      verifRepo,
		mailService:    mail,
		jwtSecret:      []byte(cfg.JWT.Secret),
		accessTTL:      cfg.JWT.AccessTTL,
		refreshTTL:     cfg.JWT.RefreshTTL,
		verifTTL:       cfg.Verification.TokenTTL,
		resendCooldown: cfg.Verification.ResendCooldown,
		maxVerifPerHr:  cfg.Verification.MaxPerHour,
		baseURL:        cfg.PublicBaseURL,
	}
}

// Register выполняет валидацию, хэширование, запись в БД и отправку письма подтверждения
func (s *AuthService) Register(ctx context.Context, username, email, password string) error {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(strings.ToLower(email))

	if username == "" || password == "" || email == "" {
		return ErrInvalidInput
	}

	if len(username) < 3 || len(username) > 30 {
		return ErrUsernameLength
	}
	if !usernameRegex.MatchString(username) {
		return ErrUsernameChars
	}

	if len(email) > 254 {
		return ErrEmailTooLong
	}
	if !emailRegex.MatchString(email) {
		return ErrEmailInvalid
	}

	if len(password) < 8 || len(password) > 72 {
		return ErrPasswordLength
	}

	dbCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	existingUser, err := s.userRepo.GetByUsernameOrEmail(dbCtx, username, email)
	if err == nil {
		if strings.EqualFold(existingUser.Username, username) {
			return ErrUsernameTaken
		}
		if strings.EqualFold(existingUser.Email, email) {
			return ErrEmailRegistered
		}
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("ошибка проверки уникальности в репозитории", "error", err)
		return ErrInternal
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("ошибка хэширования bcrypt", "error", err)
		return ErrInternal
	}

	newUser := models.User{
		Username:     username,
		PasswordHash: string(hashedPassword),
		Email:        email,
	}

	err = s.userRepo.Create(dbCtx, &newUser)
	if err != nil {
		// Обрабатываем гонку: пока шла проверка уникальности, параллельный запрос
		// мог вставить такой же username/email. Ловим нарушение UNIQUE (код 23505).
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch {
			case strings.Contains(pgErr.ConstraintName, "username"):
				return ErrUsernameTaken
			case strings.Contains(pgErr.ConstraintName, "email"):
				return ErrEmailRegistered
			default:
				return ErrUsernameTaken
			}
		}
		slog.Error("не удалось зарегистрировать пользователя в БД",
			"username", username,
			"error", err,
		)
		return ErrInternal
	}

	// Выпускаем токен подтверждения email (opaque, хранится только хеш) и
	// отправляем письмо. Ошибка отправки не отменяет регистрацию — пользователь
	// сможет запросить повторную отправку через /resend-verification.
	if err := s.issueVerification(dbCtx, &newUser); err != nil {
		slog.Error("не удалось выпустить токен верификации при регистрации",
			"user_id", newUser.ID,
			"error", err,
		)
		// Не роняем регистрацию из-за письма, но это стоит знать.
	}

	slog.Info("пользователь зарегистрирован", "username", username, "user_id", newUser.ID)
	return nil
}

// issueVerification создаёт новый токен подтверждения, сохраняет в БД только
// его SHA-256 хеш и синхронно отправляет письмо со ссылкой. Предназначен для
// регистрации и повторной отправки.
func (s *AuthService) issueVerification(ctx context.Context, user *models.User) error {
	// Инвалидируем прежние активные токены этого пользователя.
	if err := s.verifRepo.InvalidateActiveForUser(ctx, user.ID); err != nil {
		return err
	}

	rawToken, err := generateSecureToken()
	if err != nil {
		return err
	}
	if err := s.verifRepo.Create(ctx, user.ID, hashToken(rawToken), time.Now().Add(s.verifTTL)); err != nil {
		return err
	}

	link := fmt.Sprintf("%s/verify?token=%s", s.baseURL, rawToken)
	if s.mailService != nil {
		if err := s.mailService.SendVerificationEmail(user.Email, user.Username, link); err != nil {
			return err
		}
		slog.Info("письмо с подтверждением отправлено", "email", user.Email)
	}
	return nil
}

// Login проверяет логин/email, сверяет хэш пароля через bcrypt и выдает токен авторизации
func (s *AuthService) Login(ctx context.Context, login, password string) (*LoginResult, error) {
	login = strings.TrimSpace(login)
	if login == "" || password == "" {
		return nil, ErrInvalidInput
	}

	dbCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	userFromDB, err := s.userRepo.GetByUsernameOrEmail(dbCtx, login, login)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAuthFailed
		}
		slog.Error("ошибка БД при авторизации", "error", err)
		return nil, ErrInternal
	}

	err = bcrypt.CompareHashAndPassword([]byte(userFromDB.PasswordHash), []byte(password))
	if err != nil {
		slog.Warn("неудачная попытка входа: неверный пароль", "username", userFromDB.Username)
		return nil, ErrAuthFailed
	}

	// Выдаём пару токенов (access + refresh).
	return s.issueTokens(dbCtx, userFromDB)
}

// issueTokens создаёт и сохраняет новую пару токенов для пользователя:
//   - access  — JWT на короткий срок (accessTTL), stateless;
//   - refresh — случайная строка, в БД пишется только её SHA-256 хеш.
func (s *AuthService) issueTokens(ctx context.Context, user *models.User) (*LoginResult, error) {
	now := time.Now()
	claims := UserClaims{
		UserID:     user.ID,
		Username:   user.Username,
		IsAdmin:    user.IsAdmin,
		IsVerified: user.IsVerified,
		// Поля jwt.RegisteredClaims подняты наверх: поле встроенного типа
		// инициализируется напрямую (промоушен имён в литерале).
		ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		IssuedAt:  jwt.NewNumericDate(now),
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		slog.Error("не удалось подписать access-токен", "username", user.Username, "error", err)
		return nil, ErrInternal
	}

	// Генерируем opaque refresh-токен и сохраняем в БД только его хеш.
	refreshToken, err := generateSecureToken()
	if err != nil {
		slog.Error("не удалось сгенерировать refresh-токен", "error", err)
		return nil, ErrInternal
	}
	if err := s.refreshRepo.Create(ctx, user.ID, hashToken(refreshToken), now.Add(s.refreshTTL)); err != nil {
		slog.Error("не удалось сохранить refresh-токен в БД", "user_id", user.ID, "error", err)
		return nil, ErrInternal
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
		Username:     user.Username,
		Email:        user.Email,
		IsVerified:   user.IsVerified,
	}, nil
}

// Refresh обновляет пару токенов по действующему refresh-токену.
// Реализует ротацию: старый refresh-токен отзывается, выдаётся новая пара.
// Если предъявлен уже отозванный токен — это признак кражи: отзываем все
// токены пользователя и отклоняем запрос.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	if refreshToken == "" {
		return nil, ErrInvalidInput
	}

	dbCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	stored, err := s.refreshRepo.GetByHash(dbCtx, hashToken(refreshToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenInvalid
		}
		slog.Error("ошибка БД при refresh", "error", err)
		return nil, ErrInternal
	}

	// Повторное использование отозванного токена — вероятная компрометация.
	if stored.IsRevoked() {
		slog.Warn("обнаружено переиспользование отозванного refresh-токена, отзываем все токены",
			"user_id", stored.UserID)
		if revErr := s.refreshRepo.RevokeAllForUser(dbCtx, stored.UserID); revErr != nil {
			slog.Error("не удалось отозвать токены пользователя", "user_id", stored.UserID, "error", revErr)
		}
		return nil, ErrTokenInvalid
	}

	if stored.IsExpired() {
		return nil, ErrTokenExpired
	}

	// Ротация: отзываем использованный токен до выдачи новой пары.
	if err := s.refreshRepo.Revoke(dbCtx, stored.ID); err != nil {
		slog.Error("не удалось отозвать старый refresh-токен", "error", err)
		return nil, ErrInternal
	}

	user, err := s.userRepo.GetByID(dbCtx, stored.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		slog.Error("ошибка БД при получении пользователя для refresh", "error", err)
		return nil, ErrInternal
	}

	return s.issueTokens(dbCtx, user)
}

// Logout отзывает предъявленный refresh-токен. Идемпотентен: если токен уже
// недействителен, операция считается успешной.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return ErrInvalidInput
	}

	dbCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	stored, err := s.refreshRepo.GetByHash(dbCtx, hashToken(refreshToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // токена нет — считаем logout выполненным
		}
		slog.Error("ошибка БД при logout", "error", err)
		return ErrInternal
	}

	return s.refreshRepo.Revoke(dbCtx, stored.ID)
}

// generateSecureToken создаёт криптографически стойкий случайный токен
// (32 байта → base64url). Универсален для refresh- и verify-токенов: сам токен
// возвращается клиенту, в БД идёт только его хеш.
func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken вычисляет SHA-256 хеш токена в hex-представлении. Токен уже
// высокоэнтропийный, поэтому дополнительная соль/bcrypt не нужны.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// VerifyEmail проверяет токен подтверждения из письма и активирует аккаунт.
// Токен opaque: ищем его SHA-256 хеш в БД, проверяем срок и статус, помечаем
// использованным и выставляем пользователю is_verified = true.
func (s *AuthService) VerifyEmail(ctx context.Context, tokenParam string) error {
	tokenParam = strings.TrimSpace(tokenParam)
	if tokenParam == "" {
		return ErrTokenInvalid
	}

	dbCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	verif, err := s.verifRepo.GetByHash(dbCtx, hashToken(tokenParam))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTokenInvalid
		}
		slog.Error("ошибка БД при верификации", "error", err)
		return ErrInternal
	}

	if verif.IsUsed() {
		return ErrTokenAlreadyUsed
	}
	if verif.IsExpired() {
		return ErrTokenExpired
	}

	user, err := s.userRepo.GetByID(dbCtx, verif.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		slog.Error("ошибка БД при получении пользователя для верификации", "error", err)
		return ErrInternal
	}

	if user.IsVerified {
		// Токен ещё действителен, но аккаунт уже подтверждён — гасим токен.
		_ = s.verifRepo.MarkUsed(dbCtx, verif.ID)
		return ErrAlreadyVerified
	}

	if err := s.userRepo.UpdateVerificationStatus(dbCtx, user.ID, true); err != nil {
		slog.Error("не удалось обновить статус верификации",
			"user_id", user.ID,
			"error", err,
		)
		return ErrInternal
	}

	// Одноразовость: токен больше недействителен.
	if err := s.verifRepo.MarkUsed(dbCtx, verif.ID); err != nil {
		slog.Warn("не удалось пометить токен верификации использованным", "verification_id", verif.ID, "error", err)
	}

	slog.Info("аккаунт верифицирован", "username", user.Username, "user_id", user.ID)
	return nil
}

// ResendVerification повторно отправляет письмо подтверждения по email.
// Реализует защиту от абьюза:
//   - cooldown: не чаще, чем раз в resendCooldown;
//   - лимит: не более maxVerifPerHr писем в час;
//   - инвалидация старых токенов при выдаче нового.
//
// Ради безопасности не раскрывает, существует ли email и верифицирован ли он:
// в «непалящих» случаях возвращает nil (нейтральный ответ), причина пишется
// только в лог. Настоящие сбои БД/SMTP возвращаются как ошибка.
func (s *AuthService) ResendVerification(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return ErrInvalidInput
	}

	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	user, err := s.userRepo.GetByUsernameOrEmail(dbCtx, email, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Нейтральный ответ — не подтверждаем существование адреса.
			slog.Info("resend-verification: аккаунт не найден", "email", email)
			return nil
		}
		slog.Error("ошибка БД при resend-verification", "error", err)
		return ErrInternal
	}

	if user.IsVerified {
		// Уже подтверждён — молча ничего не делаем (нейтральный ответ).
		slog.Info("resend-verification: аккаунт уже верифицирован", "user_id", user.ID)
		return nil
	}

	// Cooldown: сравниваем время последней отправки с текущим моментом.
	lastSent, err := s.verifRepo.LastCreatedAt(dbCtx, user.ID)
	if err != nil {
		slog.Error("ошибка БД при проверке cooldown верификации", "error", err)
		return ErrInternal
	}
	if !lastSent.IsZero() && time.Since(lastSent) < s.resendCooldown {
		slog.Info("resend-verification: сработал cooldown", "user_id", user.ID)
		return nil
	}

	// Лимит частоты: не более maxVerifPerHr писем за последний час.
	count, err := s.verifRepo.CountSince(dbCtx, user.ID, time.Now().Add(-time.Hour))
	if err != nil {
		slog.Error("ошибка БД при подсчёте лимита верификации", "error", err)
		return ErrInternal
	}
	if count >= s.maxVerifPerHr {
		slog.Warn("resend-verification: превышен лимит писем в час", "user_id", user.ID, "count", count)
		return nil
	}

	// Всё хорошо — выпускаем новый токен (старые инвалидируются внутри) и шлём письмо.
	if err := s.issueVerification(dbCtx, user); err != nil {
		slog.Error("не удалось отправить повторное письмо верификации",
			"user_id", user.ID,
			"error", err,
		)
		return ErrInternal
	}

	slog.Info("отправлено повторное письмо верификации", "user_id", user.ID, "email", user.Email)
	return nil
}
