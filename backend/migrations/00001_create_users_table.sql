-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id SERIAL PRIMARY KEY,                                   -- Соответствует ID int
    username VARCHAR(30) UNIQUE NOT NULL,                    -- Соответствует Username string
    avatar_url TEXT DEFAULT '',                              -- Соответствует AvatarURL string
    email VARCHAR(254) UNIQUE NOT NULL,                      -- Соответствует Email string
    password_hash VARCHAR(60) NOT NULL,                      -- Соответствует PasswordHash string
    is_verified BOOLEAN DEFAULT FALSE NOT NULL,              -- Соответствует IsVerified bool
    is_admin BOOLEAN DEFAULT FALSE NOT NULL,                 -- Соответствует IsAdmin bool
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL, -- Соответствует CreatedAt time.Time
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL  -- Соответствует UpdatedAt time.Time
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
