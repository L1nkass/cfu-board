package models

import "time"

type Like struct {
	ID        int
	UserID    int
	PostID    int
	CreatedAt time.Time
}
