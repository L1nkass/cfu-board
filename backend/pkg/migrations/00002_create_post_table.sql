-- +goose Up
-- +goose StatementBegin
-- Создаем таблицу постов в строгом соответствии с моделью Go
CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    author_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Каскадное удаление: если юзер КФУ сносит аккаунт, его посты стираются автоматически
    CONSTRAINT fk_post_author FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Создаем высокопроизводительный индекс для моментального рендеринга ленты статей/объявлений
CREATE INDEX idx_posts_author_created ON posts(author_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- КОНВЕЙЕР ОТКАТА: Полностью и безопасно зачищаем систему в случае даунгрейда
-- Благодаря CASCADE, все связанные индексы и связи сотрутся без багов
DROP TABLE IF EXISTS posts CASCADE;
-- +goose StatementEnd
