package service

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	"cfu-board/pkg/config"

	"github.com/wneessen/go-mail"
)

// MailService описывает бизнес-логику для работы с почтой
type MailService interface {
	SendVerificationEmail(toEmail, username, link string) error
}

// mailService хранит в себе уже готовую конфигурацию, чтобы не читать .env каждый раз
type mailService struct {
	smtp config.SMTPConfig
}

// NewMailService — конструктор. Вызывается один раз при старте приложения.
// Принимает готовую конфигурацию SMTP (проброс зависимостей).
func NewMailService(smtp config.SMTPConfig) MailService {
	return &mailService{smtp: smtp}
}

// SendVerificationEmail отправляет красивое HTML-письмо со ссылкой для верификации
func (s *mailService) SendVerificationEmail(toEmail, username, link string) error {
	// Подушка безопасности: если SMTP не настроен, не роняем регистрацию,
	// а пишем ссылку в лог (в продакшене это предупреждение).
	if s.smtp.User == "" || s.smtp.Password == "" {
		slog.Warn("SMTP не настроен, письмо не отправлено",
			"to", toEmail,
			"verification_link", link,
		)
		return nil
	}

	// 1. Создаем почтовое сообщение
	m := mail.NewMsg()
	if err := m.From(s.smtp.User); err != nil {
		return fmt.Errorf("не удалось установить отправителя: %w", err)
	}
	if err := m.To(toEmail); err != nil {
		return fmt.Errorf("не удалось установить получателя: %w", err)
	}

	// Тема письма
	m.Subject("Подтверждение регистрации на CFU-Board 🚀")

	// Красивая HTML-верстка письма с зеленой кнопкой
	htmlBody := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; padding: 20px; max-width: 600px; border: 1px solid #eee; border-radius: 8px; margin: 0 auto;">
			<h2 style="color: #333; text-align: center;">Привет, %s! 👋</h2>
			<p style="font-size: 16px; color: #555; line-height: 1.5;">Спасибо за регистрацию в соцсети <b>CFU-Board</b>.</p>
			<p style="font-size: 16px; color: #555; line-height: 1.5;">Чтобы активировать свой аккаунт и войти на сайт, пожалуйста, подтвердите ваш Email, нажав на кнопку ниже:</p>
			<div style="margin: 30px 0; text-align: center;">
				<a href="%s" style="background-color: #28a745; color: white; padding: 14px 28px; text-decoration: none; border-radius: 5px; font-weight: bold; font-size: 16px; display: inline-block; box-shadow: 0 4px 6px rgba(0,0,0,0.1);">👉 Подтвердить аккаунт</a>
			</div>
			<p style="font-size: 12px; color: #999; text-align: center; border-top: 1px solid #eee; padding-top: 15px; margin-top: 20px;">
				Ссылка действительна в течение 15 минут.<br>Если вы не регистрировались на нашем сайте, просто проигнорируйте это письмо.
			</p>
		</div>
	`, username, link)

	m.SetBodyString(mail.TypeTextHTML, htmlBody)

	// 2. Настраиваем безопасный SMTP-клиент
	client, err := mail.NewClient(s.smtp.Host,
		mail.WithPort(s.smtp.Port),
		mail.WithSSL(), // неявный SSL, обязательно для 465
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(s.smtp.User),
		mail.WithPassword(s.smtp.Password),
		mail.WithTLSConfig(&tls.Config{ServerName: s.smtp.Host}),
		mail.WithTimeout(15*time.Second),
	)
	if err != nil {
		return fmt.Errorf("не удалось создать SMTP клиент: %w", err)
	}

	// 3. Отправляем письмо
	if err := client.DialAndSend(m); err != nil {
		return fmt.Errorf("ошибка отправки по SMTP: %w", err)
	}

	return nil
}
