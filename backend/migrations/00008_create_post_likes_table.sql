-- +goose Up
-- +goose StatementBegin
-- Лайки публикаций: один пользователь — не более одного лайка на пост
-- (гарантируется первичным ключом (post_id, user_id) вместо отдельного UNIQUE).
-- Каскад: удалили пост или пользователя — его лайки удаляются автоматически,
-- счётчик лайков удаляемого поста пересчитывать не нужно.
CREATE TABLE post_likes (
    post_id INT NOT NULL,
    user_id INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_like_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    CONSTRAINT fk_like_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT pk_post_likes PRIMARY KEY (post_id, user_id)
);

-- Индекс по user_id: PK (post_id, user_id) покрывает чтения «лайки поста»,
-- а этот нужен для «какие посты лайкнул пользователь» и быстрых каскадов по user.
CREATE INDEX idx_post_likes_user ON post_likes(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS post_likes;
-- +goose StatementEnd