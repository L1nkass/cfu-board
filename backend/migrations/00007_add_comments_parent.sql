-- +goose Up
-- +goose StatementBegin
-- Ответы на комментарии: parent_id NULL — корневой комментарий, иначе — ответ.
-- Каскад: при удалении корневого комментария его ответы удаляются вместе с ним.
ALTER TABLE comments
    ADD COLUMN parent_id INT NULL
    CONSTRAINT fk_comment_parent REFERENCES comments(id) ON DELETE CASCADE;

-- Индекс для быстрой выдачи всех ответов на конкретные корневые комментарии
CREATE INDEX idx_comments_parent ON comments(parent_id) WHERE parent_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_comments_parent;
ALTER TABLE comments DROP CONSTRAINT IF EXISTS fk_comment_parent;
ALTER TABLE comments DROP COLUMN IF EXISTS parent_id;
-- +goose StatementEnd