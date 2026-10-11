package repository

import (
	"context"
	"time"

	"cfu-board/pkg/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EmailVerificationRepository управляет токенами подтверждения email.
// В БД хранятся только SHA-256 хеши токенов.
type EmailVerificationRepository struct {
	pool *pgxpool.Pool
}

// NewEmailVerificationRepository создаёт репозиторий токенов верификации.
func NewEmailVerificationRepository(pool *pgxpool.Pool) *EmailVerificationRepository {
	return &EmailVerificationRepository{pool: pool}
}

// Create сохраняет новый токен подтверждения (только его хеш).
func (r *EmailVerificationRepository) Create(ctx context.Context, userID int, tokenHash string, expiresAt time.Time) error {
	query := `INSERT INTO email_verifications (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`
	_, err := r.pool.Exec(ctx, query, userID, tokenHash, expiresAt)
	return err
}

// GetByHash находит токен по его SHA-256 хешу. Возвращает pgx.ErrNoRows,
// если токен не найден.
func (r *EmailVerificationRepository) GetByHash(ctx context.Context, tokenHash string) (*models.EmailVerification, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at, used_at
		FROM email_verifications
		WHERE token_hash = $1
		LIMIT 1`

	var e models.EmailVerification
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&e.ID, &e.UserID, &e.TokenHash, &e.ExpiresAt, &e.CreatedAt, &e.UsedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// MarkUsed помечает конкретный токен как использованный (идемпотентно).
func (r *EmailVerificationRepository) MarkUsed(ctx context.Context, id int) error {
	query := `UPDATE email_verifications SET used_at = NOW() WHERE id = $1 AND used_at IS NULL`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

// InvalidateActiveForUser помечает все активные (неиспользованные) токены
// пользователя как использованные. Вызывается при повторной отправке письма,
// чтобы старые ссылки перестали работать.
func (r *EmailVerificationRepository) InvalidateActiveForUser(ctx context.Context, userID int) error {
	query := `UPDATE email_verifications SET used_at = NOW() WHERE user_id = $1 AND used_at IS NULL`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

// LastCreatedAt возвращает время создания самого свежего токена пользователя.
// Если токенов нет — возвращает zero time и nil.
func (r *EmailVerificationRepository) LastCreatedAt(ctx context.Context, userID int) (time.Time, error) {
	query := `SELECT COALESCE(MAX(created_at), '0001-01-01 00:00:00+00') FROM email_verifications WHERE user_id = $1`
	var ts time.Time
	err := r.pool.QueryRow(ctx, query, userID).Scan(&ts)
	return ts, err
}

// CountSince считает токены, созданные пользователем не позднее момента since.
// Используется для ограничения частоты отправок (лимит писем в час).
func (r *EmailVerificationRepository) CountSince(ctx context.Context, userID int, since time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM email_verifications WHERE user_id = $1 AND created_at >= $2`
	var count int
	err := r.pool.QueryRow(ctx, query, userID, since).Scan(&count)
	return count, err
}
