package models

import "time"

type View struct {
	ID        int
	PostID    int
	UserID    int
	IP        string
	CreatedAt time.Time
}
