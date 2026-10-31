-- +goose Up
-- +goose StatementBegin
-- Лайки комментариев: один пользователь — не более одного лайка на комментарий
-- (гарантируется первичным ключом (comment_id, user_id) вместо отдельного UNIQUE).
-- Каскад: удалили комментарий или пользователя — его лайки удаляются автоматически.
-- Удаление корневого комментария каскадом стирает ответы (FK parent_id ON DELETE
-- CASCADE), а их лайки — транзитивно через fk_comment_like_comment.
CREATE TABLE comment_likes (
    comment_id INT NOT NULL,
    user_id INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_comment_like_comment FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE,
    CONSTRAINT fk_comment_like_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT pk_comment_likes PRIMARY KEY (comment_id, user_id)
);

-- Индекс по user_id: PK (comment_id, user_id) покрывает чтения «лайки комментария»,
-- а этот нужен для «какие комментарии лайкнул пользователь» и быстрых каскадов по user.
CREATE INDEX idx_comment_likes_user ON comment_likes(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS comment_likes;
-- +goose StatementEnd
