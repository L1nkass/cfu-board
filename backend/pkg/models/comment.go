package models

import "time"

// Comment описывает комментарий пользователя под публикацией.
// ParentID == nil — корневой комментарий, иначе — ответ на комментарий с этим ID.
type Comment struct {
	ID        int       `json:"id"`
	PostID    int       `json:"post_id"`
	AuthorID  int       `json:"author_id"`
	Content   string    `json:"content"`
	ParentID  *int      `json:"parent_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CommentWithAuthor — комментарий, обогащённый публичными данными автора
// (имя и аватар). Email и прочие приватные поля сюда не попадают: структура
// наполняется из JOIN-запроса только по нужным столбцам.
// В списке комментариев фигурируют только корневые элементы; ответы на них
// лежат во вложенном срезе Replies (дерево глубиной ровно 2 уровня).
// LikesCount/LikedByMe заполняются при выдаче через GetCommentsByPostID и
// CreateCommentWithAuthor (для созданного комментария оба нулевые).
type CommentWithAuthor struct {
	ID           int                 `json:"id"`
	PostID       int                 `json:"post_id"`
	AuthorID     int                 `json:"author_id"`
	AuthorName   string              `json:"author_name"`
	AuthorAvatar string              `json:"author_avatar"`
	Content      string              `json:"content"`
	ParentID     *int                `json:"parent_id"`
	Replies      []CommentWithAuthor `json:"replies"`
	LikesCount   int                 `json:"likes_count"` // количество лайков комментария
	LikedByMe    bool                `json:"liked_by_me"` // лайкнул ли комментарий текущий зритель (false для анонимов)
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}
