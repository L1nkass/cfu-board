package repository

import (
	"cfu-board/pkg/models"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// МЕТОД СОХРАНЕНИЯ (Create)
func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	query := `
		INSERT INTO users (username, email, password_hash) 
		VALUES ($1, $2, $3)
		RETURNING id, is_verified, is_admin, created_at, updated_at;`

	// Выполняем QueryRow вместо Exec, чтобы база вернула автоматически созданные поля (ID, даты)
	// и мы сразу записали их обратно в нашу структуру Go через .Scan()
	err := r.pool.QueryRow(ctx, query, u.Username, u.Email, u.PasswordHash).Scan(
		&u.ID,
		&u.IsVerified,
		&u.IsAdmin,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	return err
}

// МЕТОД ПОИСКА логина или email (GetByUsernameOrEmail)
func (r *UserRepository) GetByUsernameOrEmail(ctx context.Context, username, email string) (*models.User, error) {
	query := `
			SELECT id, username, avatar_url, email, password_hash, is_verified, is_admin, created_at, updated_at 
			FROM users 
			WHERE LOWER(username) = LOWER($1) OR email = $2 LIMIT 1
`

	var u models.User
	err := r.pool.QueryRow(ctx, query, username, email).Scan(
		&u.ID, &u.Username, &u.AvatarURL, &u.Email, &u.PasswordHash,
		&u.IsVerified, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err // Если ничего не найдено, вернет pgx.ErrNoRows
	}
	return &u, nil
}

// МЕТОД УДАЛЕНИЯ (Delete)
func (r *UserRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM users WHERE id = $1;`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

// GetByID ищет пользователя по его уникальному ID
func (r *UserRepository) GetByID(ctx context.Context, id int) (*models.User, error) {
	query := `SELECT id, username, avatar_url, email, password_hash, is_verified, is_admin, created_at, updated_at 
			  FROM users WHERE id = $1 LIMIT 1`

	var u models.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Username, &u.AvatarURL, &u.Email, &u.PasswordHash,
		&u.IsVerified, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateVerificationStatus меняет флаг is_verified на true в базе данных
func (r *UserRepository) UpdateVerificationStatus(ctx context.Context, id int, status bool) error {
	query := `UPDATE users SET is_verified = $1, updated_at = NOW() WHERE id = $2;`
	_, err := r.pool.Exec(ctx, query, status, id)
	return err
}
