-- +goose Up
-- +goose StatementBegin
-- Таблица долгоживущих refresh-токенов.
-- ВАЖНО: хранится только SHA-256 хеш токена (token_hash), сам токен
-- отдаётся клиенту один раз и никогда не пишется в БД в открытом виде.
CREATE TABLE refresh_tokens (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    token_hash VARCHAR(64) UNIQUE NOT NULL,   -- SHA-256 в hex (64 символа)
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    -- Каскадное удаление: удалили пользователя — удалились его токены.
    CONSTRAINT fk_refresh_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS refresh_tokens;
-- +goose StatementEnd
