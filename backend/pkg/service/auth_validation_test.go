package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"cfu-board/pkg/config"
)

// newSvcForValidation собирает AuthService с nil-репозиториями: он используется
// только для проверки чистых веток валидации, которые возвращают ошибку ДО
// любого обращения к БД. Обращение к nil-пулу в этих тестах не происходит.
func newSvcForValidation() *AuthService {
	cfg := &config.Config{
		Env: config.EnvDevelopment,
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			AccessTTL:  15 * time.Minute,
			RefreshTTL: 24 * time.Hour,
		},
		Verification: config.VerificationConfig{
			TokenTTL:       time.Hour,
			ResendCooldown: 60 * time.Second,
			MaxPerHour:     3,
		},
		PublicBaseURL: "http://localhost:8080",
	}
	// NewAuthService не обращается к репозиториям в конструкторе.
	return NewAuthService(nil, nil, nil, nil, cfg)
}

// TestRegister_Validation покрывает все ветки валидации Register, срабатывающие
// до записи в БД.
func TestRegister_Validation(t *testing.T) {
	svc := newSvcForValidation()
	ctx := context.Background()

	// Корректные входные данные (проверка формата) — валидацию проходят,
	// но затем упёрлись бы в nil-БД. Поэтому здесь проверяем только то, что
	// ошибка НЕ относится к классу «валидация» (т.е. валидация пройдена).
	// Чтобы не дойти до nil-паники, проверяем отдельно каждую ошибку валидации.

	cases := []struct {
		name     string
		username string
		email    string
		password string
		wantErr  error
	}{
		{"все поля пустые", "", "", "", ErrInvalidInput},
		{"пустой username", "   ", "a@b.co", "password123", ErrInvalidInput},
		{"пустой email", "alice", "", "password123", ErrInvalidInput},
		{"пустой пароль", "alice", "a@b.co", "", ErrInvalidInput},
		{"username слишком короткий", "ab", "a@b.co", "password123", ErrUsernameLength},
		{"username слишком длинный", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "a@b.co", "password123", ErrUsernameLength}, // 31 символ
		{"username с недопустимыми символами", "alice!", "a@b.co", "password123", ErrUsernameChars},
		{"email слишком дликий", "alice", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa@b.co", "password123", ErrEmailTooLong},
		{"email некорректен", "alice", "not-an-email", "password123", ErrEmailInvalid},
		{"пароль слишком короткий", "alice", "a@b.co", "short", ErrPasswordLength},
		{"пароль слишком длинный", "alice", "a@b.co", string(make([]byte, 73)), ErrPasswordLength},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.Register(ctx, tc.username, tc.email, tc.password)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ожидалась ошибка %v, получено %v", tc.wantErr, err)
			}
		})
	}
}

// TestRegister_ValidationPasses убеждается, что полностью валидные данные
// проходят валидацию (ошибка валидации не возвращается). Дальше Register
// обращается к БД через nil-репозиторий — это приведёт к панике, которую мы
// ловим и трактуем как «валидация пройдена, дошли до слоя БД».
func TestRegister_ValidationPasses(t *testing.T) {
	svc := newSvcForValidation()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("ожидалась паника при обращении к nil-репозиторию (значит валидация пройдена, но слой БД недоступен)")
		}
	}()

	// Валидные данные должны пройти все проверки и дойти до userRepo (nil),
	// что вызовет панику — это подтверждает, что валидация их пропустила.
	_ = svc.Register(context.Background(), "alice", "alice@example.com", "password123")
}

// TestResendVerification_EmptyEmail покрывает ветку: пустой email возвращает
// ErrInvalidInput до обращения к БД.
func TestResendVerification_EmptyEmail(t *testing.T) {
	svc := newSvcForValidation()
	if err := svc.ResendVerification(context.Background(), "   "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ожидался ErrInvalidInput для пустого email, получено %v", err)
	}
}
