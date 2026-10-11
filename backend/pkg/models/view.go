package models

import "time"

// View — ЗАГЛУШКА будущей фичи счётчика просмотров (не используется: таблицы
// views нет). Оставлена как маркер: просмотры с антинакруткой (по IP/user,
// дедупликация) — отдельный этап.
type View struct {
	ID        int
	PostID    int
	UserID    int
	IP        string
	CreatedAt time.Time
}
