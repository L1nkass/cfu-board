package models

// LikeResponse — JSON-ответ операций с лайком комментария: актуальное
// количество лайков после выполненного действия.
type LikeResponse struct {
	LikesCount int `json:"likes_count"`
}

// PostLikeResponse — JSON-ответ операций с лайком поста: актуальное
// количество лайков и просмотров после выполненного действия.
type PostLikeResponse struct {
	LikesCount int `json:"likes_count"`
	ViewsCount int `json:"views_count"`
}