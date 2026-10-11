package models

import "time"

type Follow struct {
	ID          int
	FollowerID  int
	FollowingID int
	CreatedAt   time.Time
}
