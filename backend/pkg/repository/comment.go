package repository

import (
	"context"

	"cfu-board/pkg/models"
)

// CreateCommentWithAuthor сохраняет комментарий к посту (опционально — ответ на
// корневой комментарий) и сразу возвращает его вместе с публичными данными автора
// (имя, аватар). INSERT и JOIN автора выполнены одним CTE-запросом — один round-trip.
// parentID == nil — корневой комментарий; иначе родитель обязан существовать, принадлежать
// тому же посту и сам быть корневым (ответ на ответ не поддерживается — дерево глубины 2).
// Проверка родителя атомарна внутри SQL (защита от гонки «родителя удалили»).
// Возвращает pgx.ErrNoRows, если пост, автор или родительский комментарий не найдены.
func (r *UserRepository) CreateCommentWithAuthor(ctx context.Context, postID, authorID int, content string, parentID *int) (*models.CommentWithAuthor, error) {
	query := `
		WITH parent_ok AS (
			SELECT id FROM comments
			WHERE id = $4::int AND post_id = $1 AND parent_id IS NULL
		),
		inserted AS (
			INSERT INTO comments (post_id, author_id, content, parent_id)
			SELECT $1, $2, $3, $4::int
			WHERE $4::int IS NULL OR EXISTS (SELECT 1 FROM parent_ok)
			RETURNING id, post_id, author_id, content, parent_id, created_at, updated_at
		)
		SELECT i.id, i.post_id, i.author_id, u.username, COALESCE(u.avatar_url, ''),
		       i.content, i.parent_id, 0, FALSE, i.created_at, i.updated_at
		FROM inserted i
		JOIN users u ON u.id = i.author_id`

	var c models.CommentWithAuthor
	err := r.pool.QueryRow(ctx, query, postID, authorID, content, parentID).Scan(
		&c.ID, &c.PostID, &c.AuthorID, &c.AuthorName, &c.AuthorAvatar,
		&c.Content, &c.ParentID, &c.LikesCount, &c.LikedByMe,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.Replies = make([]models.CommentWithAuthor, 0)
	return &c, nil
}

// GetCommentsByPostID возвращает дерево комментариев поста: корневые комментарии
// в хронологическом порядке (старые сверху), у каждого — встроенный список ответов.
// Пагинация (limit/offset) применяется только к корневым комментариям, ответы
// дозапрашиваются одним запросом по ID корней. LIMIT явный, чтобы защитить БД
// от раздувания ответа.
// viewerID <= 0 — анонимный просмотр (liked_by_me всегда false).
func (r *UserRepository) GetCommentsByPostID(ctx context.Context, postID, limit, offset, viewerID int) ([]models.CommentWithAuthor, error) {
	rootsQuery := `
		SELECT c.id, c.post_id, c.author_id, u.username, COALESCE(u.avatar_url, ''),
		       c.content, c.parent_id,
		       (SELECT COUNT(*) FROM comment_likes cl WHERE cl.comment_id = c.id),
		       EXISTS (SELECT 1 FROM comment_likes cl WHERE cl.comment_id = c.id AND cl.user_id = $4),
		       c.created_at, c.updated_at
		FROM comments c
		JOIN users u ON u.id = c.author_id
		WHERE c.post_id = $1 AND c.parent_id IS NULL
		ORDER BY c.created_at ASC, c.id ASC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, rootsQuery, postID, limit, offset, viewerID)
	if err != nil {
		return nil, err
	}
	roots := make([]models.CommentWithAuthor, 0)
	for rows.Next() {
		var c models.CommentWithAuthor
		if err := rows.Scan(
			&c.ID, &c.PostID, &c.AuthorID, &c.AuthorName, &c.AuthorAvatar,
			&c.Content, &c.ParentID, &c.LikesCount, &c.LikedByMe,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			rows.Close()
			return nil, err
		}
		c.Replies = make([]models.CommentWithAuthor, 0)
		roots = append(roots, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	if len(roots) == 0 {
		return roots, nil
	}

	// Карта строится только после полного чтения: указатели на элементы среза
	// валидны, лишь пока backing array не перераспределяется следующим append.
	byID := make(map[int]*models.CommentWithAuthor, len(roots))
	ids := make([]int, 0, len(roots))
	for i := range roots {
		byID[roots[i].ID] = &roots[i]
		ids = append(ids, roots[i].ID)
	}
	repliesQuery := `
		SELECT c.id, c.post_id, c.author_id, u.username, COALESCE(u.avatar_url, ''),
		       c.content, c.parent_id,
		       (SELECT COUNT(*) FROM comment_likes cl WHERE cl.comment_id = c.id),
		       EXISTS (SELECT 1 FROM comment_likes cl WHERE cl.comment_id = c.id AND cl.user_id = $2),
		       c.created_at, c.updated_at
		FROM comments c
		JOIN users u ON u.id = c.author_id
		WHERE c.parent_id = ANY($1)
		ORDER BY c.created_at ASC, c.id ASC`

	repRows, err := r.pool.Query(ctx, repliesQuery, ids, viewerID)
	if err != nil {
		return nil, err
	}
	defer repRows.Close()
	for repRows.Next() {
		var c models.CommentWithAuthor
		if err := repRows.Scan(
			&c.ID, &c.PostID, &c.AuthorID, &c.AuthorName, &c.AuthorAvatar,
			&c.Content, &c.ParentID, &c.LikesCount, &c.LikedByMe,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		// Ответы — листья дерева (глубина 2): вложенных replies у них нет,
		// но поле обязано быть [] а не null, иначе фронт получит null
		// вместо массива и упадёт на .map/.length.
		c.Replies = make([]models.CommentWithAuthor, 0)
		if c.ParentID == nil {
			continue
		}
		if parent, ok := byID[*c.ParentID]; ok {
			parent.Replies = append(parent.Replies, c)
		}
	}
	if err := repRows.Err(); err != nil {
		return nil, err
	}
	return roots, nil
}

// AddCommentLike ставит лайк комментарию от имени пользователя. Идемпотентно:
// повторный лайк не создаёт дубликат (PK (comment_id, user_id) + ON CONFLICT
// DO NOTHING) и не является ошибкой. Возвращает актуальное количество лайков
// комментария после операции. Если комментарий не существует — ошибка FK
// (pgcode 23503), хендлер трактует её как 404.
func (r *UserRepository) AddCommentLike(ctx context.Context, commentID, userID int) (int, error) {
	query := `
		WITH ins AS (
			INSERT INTO comment_likes (comment_id, user_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
			RETURNING comment_id
		)
		SELECT (SELECT COUNT(*) FROM comment_likes WHERE comment_id = $1)
		     + (SELECT COUNT(*) FROM ins)`
	var count int
	err := r.pool.QueryRow(ctx, query, commentID, userID).Scan(&count)
	return count, err
}

// RemoveCommentLike снимает лайк комментария. Идемпотентно: лайка и не было —
// просто возвращается текущий счётчик без ошибки.
func (r *UserRepository) RemoveCommentLike(ctx context.Context, commentID, userID int) (int, error) {
	query := `
		WITH del AS (
			DELETE FROM comment_likes
			WHERE comment_id = $1 AND user_id = $2
			RETURNING comment_id
		)
		SELECT (SELECT COUNT(*) FROM comment_likes WHERE comment_id = $1)
		     - (SELECT COUNT(*) FROM del)`
	var count int
	err := r.pool.QueryRow(ctx, query, commentID, userID).Scan(&count)
	return count, err
}

// GetCommentByID загружает комментарий по идентификатору. Возвращает
// pgx.ErrNoRows, если комментарий не найден. Используется в хендлере удаления,
// чтобы отличить «не существует» (404) от «чужой» (403), и при создании ответа
// для детализации ошибки невалидного parent_id.
func (r *UserRepository) GetCommentByID(ctx context.Context, id int) (*models.Comment, error) {
	query := `SELECT id, post_id, author_id, content, parent_id, created_at, updated_at
		FROM comments WHERE id = $1 LIMIT 1`

	var c models.Comment
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.PostID, &c.AuthorID, &c.Content, &c.ParentID, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// DeleteCommentByID удаляет комментарий, если пользователь actorID имеет на это
// право: он автор комментария либо автор поста, под которым тот оставлен.
// Условие прав зашито в сам SQL (атомарная защита от гонок). Возвращает false,
// если подходящей записи не нашлось (нет комментария либо нет прав).
func (r *UserRepository) DeleteCommentByID(ctx context.Context, id, actorID int) (bool, error) {
	query := `
		DELETE FROM comments
		WHERE id = $1
		  AND (
		    author_id = $2
		    OR EXISTS (
		      SELECT 1 FROM posts p
		      WHERE p.id = comments.post_id AND p.author_id = $2
		    )
		  )`
	res, err := r.pool.Exec(ctx, query, id, actorID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}
