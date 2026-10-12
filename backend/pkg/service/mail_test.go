package service

import (
	"testing"

	"cfu-board/pkg/config"
)

// TestMailService_NoSMTP проверяет подушку безопасности: без настроек SMTP
// регистрация не падает — ссылка уходит в лог, возвращается nil.
func TestMailService_NoSMTP(t *testing.T) {
	m := NewMailService(config.SMTPConfig{})
	if err := m.SendVerificationEmail("user@example.com", "tester", "http://x/verify?t=1"); err != nil {
		t.Fatalf("без SMTP ожидался nil, получено %v", err)
	}
}

// TestMailService_BadFrom проверяет ошибку невалидного отправителя:
// подставляем пароль, но кривой From — go-mail должен отклонить адрес.
func TestMailService_BadFrom(t *testing.T) {
	m := NewMailService(config.SMTPConfig{
		Host:     "localhost",
		Port:     1,
		User:     "not-an-email",
		Password: "secret",
	})
	if err := m.SendVerificationEmail("user@example.com", "tester", "http://x/verify?t=1"); err == nil {
		t.Fatal("ожидалась ошибка невалидного отправителя")
	}
}
