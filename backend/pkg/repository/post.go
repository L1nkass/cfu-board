package repository

import (
	"context"
	"database/sql"

	"cfu-board/pkg/models"
)

// CreatePostWithMedia сохраняет публикацию вместе со ссылками на медиафайлы
func (r *UserRepository) CreatePostWithMedia(ctx context.Context, authorID int, title, content, imageURL, videoURL string) error {
	query := "INSERT INTO posts (author_id, title, content, image_url, video_url) VALUES ($1, $2, $3, $4, $5)"
	_, err := r.pool.Exec(ctx, query, authorID, title, content,
		sqlNullString(imageURL), sqlNullString(videoURL))
	return err
}

// Вспомогательная функция для корректного сохранения NULL в базу, если медиа нет
func sqlNullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// GetPostByID загружает публикацию по её идентификатору. Возвращает pgx.ErrNoRows,
// если пост не найден. Пустые image_url/video_url из БД маппятся в пустые строки.
func (r *UserRepository) GetPostByID(ctx context.Context, id int) (*models.Post, error) {
	query := `SELECT id, author_id, title, content, image_url, video_url, created_at, updated_at
		FROM posts WHERE id = $1 LIMIT 1`

	var (
		p        models.Post
		imageURL sql.NullString
		videoURL sql.NullString
	)
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.AuthorID, &p.Title, &p.Content,
		&imageURL, &videoURL, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.ImageURL = imageURL.String
	p.VideoURL = videoURL.String
	return &p, nil
}

// DeletePostByID удаляет публикацию, только если она принадлежит указанному автору.
// Условие авторства зашито в сам SQL-запрос, поэтому «чужой» пост удалить нельзя
// даже при гонке запросов. Возвращает false, если подходящей записи не нашлось
// (поста нет либо он принадлежит другому пользователю).
func (r *UserRepository) DeletePostByID(ctx context.Context, id, authorID int) (bool, error) {
	query := `DELETE FROM posts WHERE id = $1 AND author_id = $2`
	res, err := r.pool.Exec(ctx, query, id, authorID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}

// CountPostsByAuthor возвращает количество публикаций указанного автора.
// Используется для отображения счётчика постов в профиле пользователя.
func (r *UserRepository) CountPostsByAuthor(ctx context.Context, authorID int) (int, error) {
	query := `SELECT COUNT(*) FROM posts WHERE author_id = $1`
	var count int
	if err := r.pool.QueryRow(ctx, query, authorID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// postsFeedSelect — общий SELECT для выдачи постов с данными автора, счётчиком
// лайков, признаком liked_by_me и счётчиком комментариев. $1 — viewerID
// (<= 0 — аноним), дальше идут параметры фильтра/пагинации.
const postsFeedSelect = `
	SELECT p.id, p.author_id, u.username, COALESCE(u.avatar_url, ''),
	       p.title, p.content, p.image_url, p.video_url,
	       (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id),
	       EXISTS (SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $1),
	       (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id),
	       p.created_at, p.updated_at
	FROM posts p
	JOIN users u ON u.id = p.author_id`

// ListPostsWithLikes возвращает ленту публикаций (новые сверху) с пагинацией
// limit/offset. viewerID <= 0 — анонимный просмотр (liked_by_me всегда false).
// Возвращает пустой (не nil) срез, чтобы фронт всегда получал [] а не null.
func (r *UserRepository) ListPostsWithLikes(ctx context.Context, viewerID, limit, offset int) ([]models.Post, error) {
	return r.listPostsFeed(ctx, viewerID, limit, offset, -1)
}

// ListPostsByAuthorWithLikes возвращает публикации одного автора (новые сверху)
// с пагинацией limit/offset. viewerID <= 0 — анонимный просмотр.
func (r *UserRepository) ListPostsByAuthorWithLikes(ctx context.Context, viewerID, authorID, limit, offset int) ([]models.Post, error) {
	return r.listPostsFeed(ctx, viewerID, limit, offset, authorID)
}

// listPostsFeed — общая реализация ленты; authorID < 0 — все авторы.
func (r *UserRepository) listPostsFeed(ctx context.Context, viewerID, limit, offset, authorID int) ([]models.Post, error) {
	query := postsFeedSelect
	args := []any{viewerID}
	if authorID >= 0 {
		query += ` WHERE p.author_id = $2 ORDER BY p.created_at DESC, p.id DESC LIMIT $3 OFFSET $4`
		args = append(args, authorID, limit, offset)
	} else {
		query += ` ORDER BY p.created_at DESC, p.id DESC LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]models.Post, 0)
	for rows.Next() {
		var (
			p        models.Post
			imageURL sql.NullString
			videoURL sql.NullString
		)
		if err := rows.Scan(
			&p.ID, &p.AuthorID, &p.AuthorName, &p.AuthorAvatar,
			&p.Title, &p.Content, &imageURL, &videoURL,
			&p.LikesCount, &p.LikedByMe, &p.CommentsCount,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		p.ImageURL = imageURL.String
		p.VideoURL = videoURL.String
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return posts, nil
}
