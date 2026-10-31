-- +goose Up
-- +goose StatementBegin
-- 🟢 КОНВЕЙЕР РАЗВЕРТЫВАНИЯ: Добавляем поля для ссылок на картинку и видео
ALTER TABLE posts ADD COLUMN image_url VARCHAR(512) DEFAULT NULL;
ALTER TABLE posts ADD COLUMN video_url VARCHAR(512) DEFAULT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 🔴 КОНВЕЙЕР ОТКАТА: Безопасно удаляем поля в случае даунгрейда
ALTER TABLE posts DROP COLUMN IF EXISTS image_url;
ALTER TABLE posts DROP COLUMN IF EXISTS video_url;
-- +goose StatementEnd
