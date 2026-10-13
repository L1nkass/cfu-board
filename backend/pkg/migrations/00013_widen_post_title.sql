-- +goose Up
-- +goose StatementBegin
-- Заголовок ограничен 255 символами в API (UTF-8). VARCHAR(255) считает байты,
-- кириллица не влезала и падала 500 на INSERT. 255 рун * 4 байта = 1020.
ALTER TABLE posts ALTER COLUMN title TYPE VARCHAR(1020);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE posts ALTER COLUMN title TYPE VARCHAR(255);
-- +goose StatementEnd
