-- +goose Up
-- +goose StatementBegin
-- Таблица токенов подтверждения email. Хранится только SHA-256 хеш токена,
-- сам токен уходит в письмо и никогда не пишется в БД в открытом виде.
-- Один пользователь может иметь несколько выпущенных токенов: used_at != NULL
-- означает, что токен больше недействителен (использован или заменён при resend).
CREATE TABLE email_verifications (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    token_hash VARCHAR(64) UNIQUE NOT NULL,   -- SHA-256 в hex (64 символа)
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    CONSTRAINT fk_email_verif_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_email_verif_user ON email_verifications(user_id);
CREATE INDEX idx_email_verif_hash ON email_verifications(token_hash);
-- Индекс для cooldown/лимитов (последняя отправка по пользователю).
CREATE INDEX idx_email_verif_user_created ON email_verifications(user_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS email_verifications;
-- +goose StatementEnd
