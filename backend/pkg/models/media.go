package models

import "time"

type Media struct {
	ID        int
	PostID    int
	FileURL   string
	Type      string
	CreatedAt time.Time
}
