-- +goose Up
-- +goose StatementBegin
-- Счётчик просмотров публикации: +1 при каждом GET /posts/{id}.
-- Простой MVP без антинакрутки (дедупликация по IP/user — отдельный этап).
ALTER TABLE posts
    ADD COLUMN views_count INT NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE posts DROP COLUMN IF EXISTS views_count;
-- +goose StatementEnd
