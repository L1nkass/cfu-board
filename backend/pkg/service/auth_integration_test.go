package service

import (
	"context"
	"database/sql"
	"net/url"
	"sync"
	"testing"
	"time"

	"cfu-board/pkg/config"
	"cfu-board/pkg/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// fakeMail перехватывает ссылку верификации вместо реальной отправки письма.
type fakeMail struct {
	mu   sync.Mutex
	link string
}

func (f *fakeMail) SendVerificationEmail(toEmail, username, link string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.link = link
	return nil
}

func (f *fakeMail) tokenFromLink(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.link == "" {
		t.Fatal("письмо верификации не было отправлено")
	}
	u, err := url.Parse(f.link)
	if err != nil {
		t.Fatalf("не удалось разобрать ссылку верификации: %v", err)
	}
	return u.Query().Get("token")
}

var (
	svcPoolOnce  sync.Once
	svcPool      *pgxpool.Pool
	svcContainer testcontainers.Container
	svcSetupErr  error
)

// newSvcHarness поднимает контейнер, миграции и пул один раз на пакет.
func newSvcHarness(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	svcPoolOnce.Do(func() {
		ctx := context.Background()
		pg, err := tcpostgres.RunContainer(ctx,
			tcpostgres.WithDatabase("svc_test"),
			tcpostgres.WithUsername("tester"),
			tcpostgres.WithPassword("tester"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).WithStartupTimeout(2*time.Minute),
			),
		)
		if err != nil {
			svcSetupErr = err
			return
		}
		svcContainer = pg
		connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			svcSetupErr = err
			return
		}
		db, err := sql.Open("pgx", connStr)
		if err != nil {
			svcSetupErr = err
			return
		}
		defer db.Close()
		_ = goose.SetDialect("postgres")
		if err := goose.Up(db, "../migrations"); err != nil {
			svcSetupErr = err
			return
		}
		svcPool, err = pgxpool.New(ctx, connStr)
		if err != nil {
			svcSetupErr = err
		}
	})
	if svcSetupErr != nil {
		t.Skipf("интеграционное окружение service недоступно, тест пропущен: %v", svcSetupErr)
	}
	cleanup := func() {
		_, _ = svcPool.Exec(context.Background(),
			`TRUNCATE users, posts, comments, post_likes, comment_likes,
			 refresh_tokens, email_verifications RESTART IDENTITY CASCADE`)
	}
	return svcPool, cleanup
}

// buildSvc собирает AuthService с реальными репозиториями и фейковым mail.
func buildSvc(pool *pgxpool.Pool) (*AuthService, *fakeMail) {
	userRepo := repository.NewUserRepository(pool)
	refreshRepo := repository.NewRefreshTokenRepository(pool)
	verifRepo := repository.NewEmailVerificationRepository(pool)
	mail := &fakeMail{}
	cfg := &config.Config{
		Env: config.EnvDevelopment,
		JWT: config.JWTConfig{Secret: "integration-secret", AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour},
		Verification: config.VerificationConfig{
			TokenTTL: time.Hour, ResendCooldown: 0, MaxPerHour: 3,
		},
		PublicBaseURL: "http://localhost:8080",
	}
	return NewAuthService(userRepo, refreshRepo, verifRepo, mail, cfg), mail
}

// TestService_AuthFlow проверяет сквозной сценарий: регистрация с письмом,
// вход, верификацию по токену из письма и повторный вход уже верифицированным.
func TestService_AuthFlow(t *testing.T) {
	pool, cleanup := newSvcHarness(t)
	cleanup()
	svc, mail := buildSvc(pool)
	ctx := context.Background()

	if err := svc.Register(ctx, "alice", "alice@example.com", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// До верификации вход проходит, но IsVerified=false.
	res, err := svc.Login(ctx, "alice", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("Login должен вернуть пару токенов")
	}
	if res.IsVerified {
		t.Fatal("до верификации IsVerified должен быть false")
	}

	// Верифицируемся по токену из перехваченного письма.
	if err := svc.VerifyEmail(ctx, mail.tokenFromLink(t)); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}

	// Повторный вход отражает подтверждённый аккаунт.
	res2, err := svc.Login(ctx, "alice@example.com", "password123")
	if err != nil {
		t.Fatalf("Login после верификации: %v", err)
	}
	if !res2.IsVerified {
		t.Fatal("после верификации IsVerified должен быть true")
	}

	// Повторная верификация тем же токеном → уже использован.
	if err := svc.VerifyEmail(ctx, mail.tokenFromLink(t)); err != ErrTokenAlreadyUsed {
		t.Fatalf("повторная верификация: ожидался ErrTokenAlreadyUsed, получено %v", err)
	}
}

// TestService_RefreshRotation проверяет ротацию refresh-токена и защиту от
// переиспользования уже отозванного токена (признак кражи → отзыв всех).
func TestService_RefreshRotation(t *testing.T) {
	pool, cleanup := newSvcHarness(t)
	cleanup()
	svc, _ := buildSvc(pool)
	ctx := context.Background()

	if err := svc.Register(ctx, "bob", "bob@example.com", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	login, err := svc.Login(ctx, "bob", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Ротация: старый токен обменивается на новую пару.
	refreshed, err := svc.Refresh(ctx, login.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if refreshed.RefreshToken == "" || refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("Refresh должен выдать новый refresh-токен")
	}

	// Переиспользование СТАРОГО (отозванного) токена — отклоняется и отзывает все.
	if _, err := svc.Refresh(ctx, login.RefreshToken); err != ErrTokenInvalid {
		t.Fatalf("переиспользование отозванного токена: ожидался ErrTokenInvalid, получено %v", err)
	}
	// Новый токен тоже отозван (кража) → refresh невозможен.
	if _, err := svc.Refresh(ctx, refreshed.RefreshToken); err != ErrTokenInvalid {
		t.Fatalf("после детекта кражи новый токен тоже должен быть отозван, получено %v", err)
	}
}

// TestService_Logout проверяет, что после Logout refresh-токен недействителен.
func TestService_Logout(t *testing.T) {
	pool, cleanup := newSvcHarness(t)
	cleanup()
	svc, _ := buildSvc(pool)
	ctx := context.Background()

	if err := svc.Register(ctx, "carol", "carol@example.com", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	login, err := svc.Login(ctx, "carol", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := svc.Logout(ctx, login.RefreshToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := svc.Refresh(ctx, login.RefreshToken); err != ErrTokenInvalid {
		t.Fatalf("после Logout refresh должен быть недействителен, получено %v", err)
	}
}

// TestService_LoginErrors покрывает ошибочные пути входа и дубликаты регистрации.
func TestService_LoginErrors(t *testing.T) {
	pool, cleanup := newSvcHarness(t)
	cleanup()
	svc, _ := buildSvc(pool)
	ctx := context.Background()

	if err := svc.Register(ctx, "dave", "dave@example.com", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if _, err := svc.Login(ctx, "dave", "wrongpass"); err != ErrAuthFailed {
		t.Fatalf("неверный пароль: ожидался ErrAuthFailed, получено %v", err)
	}
	if _, err := svc.Login(ctx, "ghost", "password123"); err != ErrAuthFailed {
		t.Fatalf("несуществующий пользователь: ожидался ErrAuthFailed, получено %v", err)
	}
	if _, err := svc.Login(ctx, "", ""); err != ErrInvalidInput {
		t.Fatalf("пустые поля: ожидался ErrInvalidInput, получено %v", err)
	}

	if err := svc.Register(ctx, "dave", "other@example.com", "password123"); err != ErrUsernameTaken {
		t.Fatalf("дубликат username: ожидался ErrUsernameTaken, получено %v", err)
	}
	if err := svc.Register(ctx, "dave2", "dave@example.com", "password123"); err != ErrEmailRegistered {
		t.Fatalf("дубликат email: ожидался ErrEmailRegistered, получено %v", err)
	}
}
