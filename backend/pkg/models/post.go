package models

import "time"

// Статусы модерации публикации.
//   - Pending  — создан автором, ждёт решения админа; в общей ленте не виден.
//   - Approved — одобрен, попадает в общую ленту.
//   - Rejected — отклонён админом; автор видит причину (RejectionReason).
const (
	PostStatusPending  = "pending"
	PostStatusApproved = "approved"
	PostStatusRejected = "rejected"
)

// Post описывает монументальную модель публикации/объявления на CFU-Board.
// LikesCount/LikedByMe заполняются только при выдаче через GET /posts/{id},
// GET /posts и GET /users/{id}/posts (GetPostWithLikes/ListPostsWithLikes);
// внутренние проверки существования/авторства этих полей не трогают.
// AuthorName/AuthorAvatar/CommentsCount заполняются там же (нужны для карточек
// ленты); во внутренних проверках остаются пустыми.
//
// Модерация: новый пост создаётся со статусом Pending и не виден в общей ленте,
// пока админ не одобрит его (Approved). ModeratedBy/ModeratedAt фиксируют, кто
// и когда принял решение; RejectionReason — причина отказа для статуса Rejected.
type Post struct {
	ID              int        `json:"id"`
	AuthorID        int        `json:"author_id"`
	AuthorName      string     `json:"author_name"`
	AuthorAvatar    string     `json:"author_avatar"`
	Title           string     `json:"title"`
	Content         string     `json:"content"`
	ImageURL        string     `json:"image_url,omitempty"` // omitempty скроет поле в JSON, если медиа нет
	VideoURL        string     `json:"video_url,omitempty"`
	Status          string     `json:"status"`                     // pending | approved | rejected
	RejectionReason string     `json:"rejection_reason,omitempty"` // причина отказа (для rejected)
	ModeratedBy     *int       `json:"moderated_by,omitempty"`     // id админа, принявшего решение
	ModeratedAt     *time.Time `json:"moderated_at,omitempty"`     // когда принято решение
	LikesCount      int        `json:"likes_count"`                // количество лайков поста
	LikedByMe       bool       `json:"liked_by_me"`                // лайкнул ли пост текущий зритель (false для анонимов)
	CommentsCount   int        `json:"comments_count"`             // количество комментариев поста
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// IsVisibleToViewer возвращает true, если пост может быть показан зрителю
// viewerID (viewerID <= 0 — аноним): одобренные посты видят все, а вот
// неодобренные (pending/rejected) — только их автор.
func (p *Post) IsVisibleToViewer(viewerID int) bool {
	return p.Status == PostStatusApproved || (viewerID > 0 && viewerID == p.AuthorID)
}
