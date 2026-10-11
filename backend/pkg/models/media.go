package models

import "time"

// Media — ЗАГЛУШКА: отдельная таблица медиа не используется, файлы поста
// хранятся полями posts.image_url/video_url + объектным хранилищем (MinIO).
// Оставлена как маркер: множественные вложения на пост — отдельный этап.
type Media struct {
	ID        int
	PostID    int
	FileURL   string
	Type      string
	CreatedAt time.Time
}
