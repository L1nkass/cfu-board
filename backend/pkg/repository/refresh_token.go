package repository

import (
	"context"
	"time"

	"cfu-board/pkg/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RefreshTokenRepository отвечает за хранение и управление refresh-токенами.
// В БД сохраняются только SHA-256 хеши токенов, что защищает от утечки
// рабочих токенов при компрометации базы.
type RefreshTokenRepository struct {
	pool *pgxpool.Pool
}

// NewRefreshTokenRepository создаёт репозиторий refresh-токенов.
func NewRefreshTokenRepository(pool *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{pool: pool}
}

// Create сохраняет новый refresh-токен (только его хеш).
func (r *RefreshTokenRepository) Create(ctx context.Context, userID int, tokenHash string, expiresAt time.Time) error {
	query := `INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`
	_, err := r.pool.Exec(ctx, query, userID, tokenHash, expiresAt)
	return err
}

// GetByHash находит токен по его SHA-256 хешу. Возвращает pgx.ErrNoRows,
// если токен не найден.
func (r *RefreshTokenRepository) GetByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1
		LIMIT 1`

	var t models.RefreshToken
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt, &t.RevokedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Revoke помечает конкретный токен как отозванный (идемпотентно: повторный
// вызов не меняет уже отозванный токен).
func (r *RefreshTokenRepository) Revoke(ctx context.Context, id int) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

// RevokeAllForUser отзывает все активные токены пользователя. Используется
// при logout со всех устройств и при обнаружении компрометации токена.
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID int) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}
