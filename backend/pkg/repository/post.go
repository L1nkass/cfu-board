package repository

import (
	"context"
	"database/sql"

	"cfu-board/pkg/models"

	"github.com/jackc/pgx/v5"
)

// CreatePostWithMedia сохраняет публикацию вместе со ссылками на медиафайлы.
// Новый пост создаётся со статусом Pending (на модерации) и НЕ попадает в общую
// ленту, пока админ не одобрит его. Возвращает ID созданной публикации.
func (r *UserRepository) CreatePostWithMedia(ctx context.Context, authorID int, title, content, imageURL, videoURL string) (int, error) {
	query := `INSERT INTO posts (author_id, title, content, image_url, video_url, status)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	var id int
	err := r.pool.QueryRow(ctx, query, authorID, title, content,
		sqlNullString(imageURL), sqlNullString(videoURL), models.PostStatusPending).Scan(&id)
	return id, err
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
// Внутренний метод (проверка существования/авторства) — НЕ фильтрует по статусу
// модерации: он должен находить пост независимо от pending/approved/rejected,
// а решение о видимости принимают вызывающие хендлеры.
func (r *UserRepository) GetPostByID(ctx context.Context, id int) (*models.Post, error) {
	query := `SELECT id, author_id, title, content, image_url, video_url,
		status, rejection_reason, moderated_by, moderated_at, views_count, created_at, updated_at
		FROM posts WHERE id = $1 LIMIT 1`

	var (
		p        models.Post
		imageURL sql.NullString
		videoURL sql.NullString
		reason   sql.NullString
		modBy    sql.NullInt64
		modAt    sql.NullTime
	)
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.AuthorID, &p.Title, &p.Content,
		&imageURL, &videoURL,
		&p.Status, &reason, &modBy, &modAt, &p.ViewsCount,
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

// postsFeedSelect — общий SELECT для выдачи постов с данными автора, полями
// модерации, счётчиком лайков, признаком liked_by_me и счётчиком комментариев.
// $1 — viewerID (<= 0 — аноним), дальше идут параметры фильтра/пагинации.
// Видимость (approved OR свой) накладывает вызывающий код через WHERE.
const postsFeedSelect = `
	SELECT p.id, p.author_id, u.username, COALESCE(u.avatar_url, ''),
	       p.title, p.content, p.image_url, p.video_url,
	       p.status, p.rejection_reason, p.moderated_by, p.moderated_at,
	       (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id),
	       EXISTS (SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $1),
	       (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id),
	       p.views_count,
	       p.created_at, p.updated_at
	FROM posts p
	JOIN users u ON u.id = p.author_id`

// ListPostsWithLikes возвращает ленту публикаций (новые сверху) с пагинацией
// limit/offset. viewerID <= 0 — анонимный просмотр (liked_by_me всегда false).
// Видимость: только одобренные посты, либо — для автора — его собственные
// неодобренные (pending/rejected). Возвращает пустой (не nil) срез.
func (r *UserRepository) ListPostsWithLikes(ctx context.Context, viewerID, limit, offset int) ([]models.Post, error) {
	return r.listPostsFeed(ctx, viewerID, limit, offset, -1)
}

// ListPostsByAuthorWithLikes возвращает публикации одного автора (новые сверху)
// с пагинацией limit/offset. viewerID <= 0 — анонимный просмотр. Видимость та же:
// прочие зрители видят только одобренные посты автора, сам автор — все свои.
func (r *UserRepository) ListPostsByAuthorWithLikes(ctx context.Context, viewerID, authorID, limit, offset int) ([]models.Post, error) {
	return r.listPostsFeed(ctx, viewerID, limit, offset, authorID)
}

// listPostsFeed — общая реализация ленты; authorID < 0 — все авторы.
func (r *UserRepository) listPostsFeed(ctx context.Context, viewerID, limit, offset, authorID int) ([]models.Post, error) {
	query := postsFeedSelect
	args := []any{viewerID}
	if authorID >= 0 {
		query += ` WHERE p.author_id = $2 AND (p.status = 'approved' OR p.author_id = $1)
			ORDER BY p.created_at DESC, p.id DESC LIMIT $3 OFFSET $4`
		args = append(args, authorID, limit, offset)
	} else {
		query += ` WHERE (p.status = 'approved' OR p.author_id = $1)
			ORDER BY p.created_at DESC, p.id DESC LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]models.Post, 0)
	for rows.Next() {
		p, err := scanFeedRow(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return posts, nil
}

// postsModerationSelect — SELECT для админ-очереди модерации (посты со статусом
// pending). liked_by_me не персонализируется (литерал FALSE), поэтому $1 свободен
// и используется под статус. Набор столбцов совпадает с postsFeedSelect, чтобы
// переиспользовать scanFeedRow.
const postsModerationSelect = `
	SELECT p.id, p.author_id, u.username, COALESCE(u.avatar_url, ''),
	       p.title, p.content, p.image_url, p.video_url,
	       p.status, p.rejection_reason, p.moderated_by, p.moderated_at,
	       (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id),
	       FALSE,
	       (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id),
	       p.views_count,
	       p.created_at, p.updated_at
	FROM posts p
	JOIN users u ON u.id = p.author_id`

// ListPendingPosts возвращает очередь модерации: посты со статусом 'pending'
// (старые сверху — FIFO), с данными автора. Предназначено для админ-панели.
// Возвращает пустой (не nil) срез.
func (r *UserRepository) ListPendingPosts(ctx context.Context, limit, offset int) ([]models.Post, error) {
	query := postsModerationSelect + ` WHERE p.status = $1
		ORDER BY p.created_at ASC, p.id ASC LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, models.PostStatusPending, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]models.Post, 0)
	for rows.Next() {
		p, err := scanFeedRow(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return posts, nil
}

// CountPendingPosts возвращает количество постов, ожидающих модерации (badge
// для админ-панели).
func (r *UserRepository) CountPendingPosts(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts WHERE status = $1`, models.PostStatusPending).Scan(&count)
	return count, err
}

// SetPostModeration обновляет статус модерации поста: одобряет ('approved') или
// отклоняет ('rejected'). Фиксирует, кто (moderatorID) и когда принял решение, а
// для отказа — причину (reason). При одобрении причина всегда очищается.
// Возвращает false, если пост с таким ID не найден.
func (r *UserRepository) SetPostModeration(ctx context.Context, postID int, status string, moderatorID int, reason string) (bool, error) {
	// Логику причины вычисляем в Go, а не в SQL: так параметр $1 используется
	// единожды (status = $1), что исключает ошибку PG 42P08 (несовпадение
	// выведенных типов одного плейсхолдера в разных контекстах prepared-stmt).
	// rejection_reason заполняется только при отказе (rejected), иначе — NULL.
	var reasonParam any
	if status == models.PostStatusRejected && reason != "" {
		reasonParam = reason
	}
	query := `UPDATE posts
		SET status = $1,
		    moderated_by = $2,
		    moderated_at = NOW(),
		    rejection_reason = $3
		WHERE id = $4`
	res, err := r.pool.Exec(ctx, query, status, moderatorID, reasonParam, postID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}

// scanFeedRow сканирует одну строку результата postsFeedSelect/postsModerationSelect
// (единый набор столбцов) в models.Post, корректно маппя NULL-поля модерации.
func scanFeedRow(rows pgx.Rows) (models.Post, error) {
	var (
		p        models.Post
		imageURL sql.NullString
		videoURL sql.NullString
		reason   sql.NullString
		modBy    sql.NullInt64
		modAt    sql.NullTime
	)
	err := rows.Scan(
		&p.ID, &p.AuthorID, &p.AuthorName, &p.AuthorAvatar,
		&p.Title, &p.Content, &imageURL, &videoURL,
		&p.Status, &reason, &modBy, &modAt,
		&p.LikesCount, &p.LikedByMe, &p.CommentsCount,
		&p.ViewsCount,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return models.Post{}, err
	}
	p.ImageURL = imageURL.String
	p.VideoURL = videoURL.String
	applyModeration(&p, reason, modBy, modAt)
	return p, nil
}

// applyModeration переносит NULL-столбцы модерации в модель Post.
func applyModeration(p *models.Post, reason sql.NullString, modBy sql.NullInt64, modAt sql.NullTime) {
	if reason.Valid {
		p.RejectionReason = reason.String
	}
	if modBy.Valid {
		v := int(modBy.Int64)
		p.ModeratedBy = &v
	}
	if modAt.Valid {
		v := modAt.Time
		p.ModeratedAt = &v
	}
}
