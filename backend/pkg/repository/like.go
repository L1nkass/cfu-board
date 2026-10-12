package repository

import (
	"context"
	"database/sql"

	"cfu-board/pkg/models"
)

// AddLike ставит лайк посту от имени пользователя. Идемпотентно: повторный
// лайк не создаёт дубликат (первичный ключ (post_id, user_id) + ON CONFLICT
// DO NOTHING) и не является ошибкой. Возвращает актуальное количество лайков
// поста после операции. Если пост не существует, запрос завершается ошибкой
// FK-нарушения (pgconn.PgError с кодом 23503) — хендлер трактует её как 404.
//
// ВАЖНО: основной SELECT и INSERT-CTE в одном statement видят один и тот же
// снимок данных, поэтому COUNT(*) напрямую не увидел бы новую строку. Число
// реально вставленных строк забираем через RETURNING и прибавляем к счётчику
// «до» — так ответ всегда актуален, и всё остаётся одним атомарным запросом.
func (r *UserRepository) AddLike(ctx context.Context, postID, userID int) (int, error) {
	query := `
		WITH ins AS (
			INSERT INTO post_likes (post_id, user_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
			RETURNING post_id
		)
		SELECT (SELECT COUNT(*) FROM post_likes WHERE post_id = $1)
		     + (SELECT COUNT(*) FROM ins)`
	var count int
	err := r.pool.QueryRow(ctx, query, postID, userID).Scan(&count)
	return count, err
}

// RemoveLike снимает лайк поста с пользователя. Идемпотентно: лайка и не было —
// просто возвращается текущий счётчик без ошибки. Возвращает актуальное
// количество лайков поста после операции. Аналогично AddLike число удалённых
// строк забирается через RETURNING (основной SELECT не видит эффект DELETE-CTE
// того же statement из-за общего снимка данных).
func (r *UserRepository) RemoveLike(ctx context.Context, postID, userID int) (int, error) {
	query := `
		WITH del AS (
			DELETE FROM post_likes
			WHERE post_id = $1 AND user_id = $2
			RETURNING post_id
		)
		SELECT (SELECT COUNT(*) FROM post_likes WHERE post_id = $1)
		     - (SELECT COUNT(*) FROM del)`
	var count int
	err := r.pool.QueryRow(ctx, query, postID, userID).Scan(&count)
	return count, err
}

// GetPostWithLikes загружает публикацию вместе с данными автора, счётчиком
// лайков, признаком «лайкнул ли этот пост текущий зритель», счётчиком
// комментариев и счётчиком просмотров. viewerID <= 0 — анонимный просмотр
// (liked_by_me всегда false: пользователей с id <= 0 в БД нет). Возвращает
// pgx.ErrNoRows, если пост не найден. Пустые image_url/video_url из БД маппятся
// в пустые строки.
//
// Модерация: неодобренные посты (pending/rejected) видит ТОЛЬКО их автор —
// фильтр видимости зашит прямо в SQL (status='approved' OR автор=зритель).
func (r *UserRepository) GetPostWithLikes(ctx context.Context, postID, viewerID int) (*models.Post, error) {
	query := `
		SELECT p.id, p.author_id, u.username, COALESCE(u.avatar_url, ''),
		       p.title, p.content, p.image_url, p.video_url,
		       p.status, p.rejection_reason, p.moderated_by, p.moderated_at,
			(SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id),
			EXISTS (SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $2),
			(SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id),
			p.views_count,
			p.created_at, p.updated_at
		FROM posts p
		JOIN users u ON u.id = p.author_id
		WHERE p.id = $1 AND (p.status = 'approved' OR p.author_id = $2)
		LIMIT 1`

	var (
		p        models.Post
		imageURL sql.NullString
		videoURL sql.NullString
		reason   sql.NullString
		modBy    sql.NullInt64
		modAt    sql.NullTime
	)
	err := r.pool.QueryRow(ctx, query, postID, viewerID).Scan(
		&p.ID, &p.AuthorID, &p.AuthorName, &p.AuthorAvatar,
		&p.Title, &p.Content, &imageURL, &videoURL,
		&p.Status, &reason, &modBy, &modAt,
		&p.LikesCount, &p.LikedByMe, &p.CommentsCount,
		&p.ViewsCount,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.ImageURL = imageURL.String
	p.VideoURL = videoURL.String
	applyModeration(&p, reason, modBy, modAt)
	return &p, nil
}

// IncrementPostViews атомарно увеличивает счётчик просмотров публикации на 1
// и возвращает актуальное значение. Вызывается при каждом GET /posts/{id}
// (best-effort: ошибка инкремента не должна ронять просмотр поста).
func (r *UserRepository) IncrementPostViews(ctx context.Context, postID int) (int, error) {
	var views int
	err := r.pool.QueryRow(ctx,
		`UPDATE posts SET views_count = views_count + 1 WHERE id = $1 RETURNING views_count`,
		postID,
	).Scan(&views)
	return views, err
}
