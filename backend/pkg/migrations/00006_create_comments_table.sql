-- +goose Up
-- +goose StatementBegin
-- Таблица комментариев к публикациям в строгом соответствии с моделью Go
CREATE TABLE comments (
    id SERIAL PRIMARY KEY,
    post_id INT NOT NULL,
    author_id INT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Каскадное удаление: комментарии стираются вместе с постом и вместе с аккаунтом автора
    CONSTRAINT fk_comment_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    CONSTRAINT fk_comment_author FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Индекс для быстрой выдачи ленты комментариев поста в хронологическом порядке
CREATE INDEX idx_comments_post_created ON comments(post_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS comments CASCADE;
-- +goose StatementEnd
