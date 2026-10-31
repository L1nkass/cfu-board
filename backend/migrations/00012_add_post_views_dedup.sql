-- +goose Up
-- +goose StatementBegin
-- Дедупликация просмотров по IP: один IP засчитывает пост только один раз.
-- views_count в posts — денормализованный счётчик (быстрое чтение для ленты),
-- а post_views — источник уникальности (post_id, ip).
CREATE TABLE post_views (
    post_id INT NOT NULL,
    ip VARCHAR(45) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_view_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    CONSTRAINT pk_post_views PRIMARY KEY (post_id, ip)
);

-- Индекс для очистки/аналитики по времени (опционально).
CREATE INDEX idx_post_views_created ON post_views(created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS post_views;
-- +goose StatementEnd
