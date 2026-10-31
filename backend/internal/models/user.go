package models

import "time"

type User struct {
	ID           int       `db:"id" json:"id"`
	Username     string    `db:"username" json:"username"`
	AvatarURL    string    `db:"avatar_url" json:"avatar_url"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	IsVerified   bool      `db:"is_verified" json:"is_verified"`
	IsAdmin      bool      `db:"is_admin" json:"is_admin"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}
