package models

import "time"

// Follow — ЗАГЛУШКА будущей фичи подписок (не используется: таблицы follows
// нет, см. миграции). Оставлена как маркер скоупа: подписки, лента
// подписок, уведомления — отдельный этап, не часть текущего API.
type Follow struct {
	ID          int
	FollowerID  int
	FollowingID int
	CreatedAt   time.Time
}
