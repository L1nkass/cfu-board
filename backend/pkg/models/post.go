package models

import "time"

// Post описывает монументальную модель публикации/объявления на CFU-Board.
// LikesCount/LikedByMe заполняются только при выдаче через GET /posts/{id},
// GET /posts и GET /users/{id}/posts (GetPostWithLikes/ListPostsWithLikes);
// внутренние проверки существования/авторства этих полей не трогают.
// AuthorName/AuthorAvatar/CommentsCount заполняются там же (нужны для карточек
// ленты); во внутренних проверках остаются пустыми.
type Post struct {
	ID           int       `json:"id"`
	AuthorID     int       `json:"author_id"`
	AuthorName   string    `json:"author_name"`
	AuthorAvatar string    `json:"author_avatar"`
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	ImageURL     string    `json:"image_url,omitempty"` // omitempty скроет поле в JSON, если медиа нет
	VideoURL     string    `json:"video_url,omitempty"`
	LikesCount   int       `json:"likes_count"`   // количество лайков поста
	LikedByMe    bool      `json:"liked_by_me"`   // лайкнул ли пост текущий зритель (false для анонимов)
	CommentsCount int      `json:"comments_count"` // количество комментариев поста
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
