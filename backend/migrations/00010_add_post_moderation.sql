-- +goose Up
-- +goose StatementBegin
-- Модерация публикаций: новый пост создаётся со статусом 'pending' (на
-- модерации) и НЕ попадает в общую ленту, пока админ не одобрит его
-- ('approved'). Отклонённые посты получают статус 'rejected' с причиной.
--
-- DEFAULT 'approved' — для обратной совместимости: уже существующие в проде
-- посты должны оставаться видимыми. Новые посты явно создаются как 'pending'
-- на стороне приложения (CreatePostWithMedia), опираясь на этот столбец.
ALTER TABLE posts
    ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'approved'
        CONSTRAINT chk_posts_status CHECK (status IN ('pending', 'approved', 'rejected'));

-- Кто и когда отмодерировал, а также причина отказа (для 'rejected').
ALTER TABLE posts
    ADD COLUMN moderated_by INT NULL
        CONSTRAINT fk_post_moderator REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN moderated_at TIMESTAMP WITH TIME ZONE NULL,
    ADD COLUMN rejection_reason TEXT NULL;

-- Индекс для быстрой выборки постов на модерации (очередь админа).
CREATE INDEX idx_posts_status_created ON posts(status, created_at DESC) WHERE status <> 'approved';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_posts_status_created;
ALTER TABLE posts DROP CONSTRAINT IF EXISTS fk_post_moderator;
ALTER TABLE posts DROP COLUMN IF EXISTS rejection_reason;
ALTER TABLE posts DROP COLUMN IF EXISTS moderated_at;
ALTER TABLE posts DROP COLUMN IF EXISTS moderated_by;
ALTER TABLE posts DROP CONSTRAINT IF EXISTS chk_posts_status;
ALTER TABLE posts DROP COLUMN IF EXISTS status;
-- +goose StatementEnd
