package repository

import (
	"context"
	"errors"
	"testing"

	"cfu-board/pkg/models"

	"github.com/jackc/pgx/v5"
)

// TestUserRepository_CreateGetDelete проверяет полный цикл жизни пользователя:
// Create (с возвращением сгенерированных полей), чтение по ID и по логину/почте
// (регистронезависимо), а также Delete с последующим ErrNoRows.
func TestUserRepository_CreateGetDelete(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	u := seedUser(t, repo, "alice", "alice@example.com")
	if u.ID == 0 {
		t.Fatal("Create не присвоил ID")
	}
	if u.IsVerified {
		t.Fatal("новый пользователь должен быть неверифицирован")
	}

	got, err := repo.GetByID(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Username != "alice" || got.Email != "alice@example.com" {
		t.Fatalf("GetByID вернул неожиданные данные: %+v", got)
	}

	// Поиск по логину регистронезависим.
	found, err := repo.GetByUsernameOrEmail(context.Background(), "ALICE", "no@example.com")
	if err != nil {
		t.Fatalf("GetByUsernameOrEmail: %v", err)
	}
	if found.ID != u.ID {
		t.Fatalf("поиск по логину нашёл пользователя %d, ожидался %d", found.ID, u.ID)
	}

	if err := repo.Delete(context.Background(), u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(context.Background(), u.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("после Delete ожидался ErrNoRows, получено %v", err)
	}
}

// TestUserRepository_UpdateVerificationStatus проверяет переключение флага
// верификации пользователя.
func TestUserRepository_UpdateVerificationStatus(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	u := seedUser(t, repo, "verif", "verif@example.com")
	if err := repo.UpdateVerificationStatus(context.Background(), u.ID, true); err != nil {
		t.Fatalf("UpdateVerificationStatus: %v", err)
	}
	got, err := repo.GetByID(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !got.IsVerified {
		t.Fatal("после верификации ожидался is_verified=true")
	}
}

// TestUserRepository_DuplicateRejected проверяет, что уникальные ограничения на
// username и email отклоняют дубликаты.
func TestUserRepository_DuplicateRejected(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	seedUser(t, repo, "bob", "bob@example.com")

	if err := repo.Create(context.Background(), &models.User{
		Username: "bob", Email: "other@example.com", PasswordHash: "x",
	}); err == nil {
		t.Fatal("ожидалась ошибка уникальности username")
	}

	if err := repo.Create(context.Background(), &models.User{
		Username: "bobby", Email: "bob@example.com", PasswordHash: "x",
	}); err == nil {
		t.Fatal("ожидалась ошибка уникальности email")
	}
}
