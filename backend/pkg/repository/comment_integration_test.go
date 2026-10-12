package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestComments_TreeCreateAndList проверяет создание корневого комментария и
// ответа, сборку дерева с вложенными Replies и обновление счётчика комментариев
// поста.
func TestComments_TreeCreateAndList(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	author := seedUser(t, repo, "cAuthor", "ca@example.com")
	commenter := seedUser(t, repo, "cCommenter", "cc@example.com")
	postID := seedPost(t, pool, author.ID, "Пост для комментариев")

	root, err := repo.CreateCommentWithAuthor(context.Background(), postID, commenter.ID, "корень", nil)
	if err != nil {
		t.Fatalf("CreateCommentWithAuthor root: %v", err)
	}
	if root.AuthorName != "cCommenter" {
		t.Fatalf("AuthorName корневого комментария: %q", root.AuthorName)
	}
	if root.Replies == nil || len(root.Replies) != 0 {
		t.Fatalf("Replies должен быть пустым срезом, получено %v", root.Replies)
	}

	reply, err := repo.CreateCommentWithAuthor(context.Background(), postID, author.ID, "ответ", &root.ID)
	if err != nil {
		t.Fatalf("CreateCommentWithAuthor reply: %v", err)
	}
	if reply.ParentID == nil || *reply.ParentID != root.ID {
		t.Fatalf("ParentID ответа: %v", reply.ParentID)
	}

	tree, err := repo.GetCommentsByPostID(context.Background(), postID, 50, 0, 0)
	if err != nil {
		t.Fatalf("GetCommentsByPostID: %v", err)
	}
	if len(tree) != 1 {
		t.Fatalf("ожидался 1 корневой комментарий, получено %d", len(tree))
	}
	if len(tree[0].Replies) != 1 {
		t.Fatalf("ожидался 1 ответ, получено %d", len(tree[0].Replies))
	}
	if tree[0].Replies[0].ID != reply.ID {
		t.Fatalf("в дереве неверный ответ: %+v", tree[0].Replies[0])
	}

	// Счётчик комментариев поста учитывает оба комментария.
	post, err := repo.GetPostWithLikes(context.Background(), postID, 0)
	if err != nil {
		t.Fatalf("GetPostWithLikes: %v", err)
	}
	if post.CommentsCount != 2 {
		t.Fatalf("CommentsCount=%d, ожидалось 2", post.CommentsCount)
	}
}

// TestComments_InvalidParentRejected проверяет защиту от «ответа на ответ» и от
// привязки к чужому/несуществующему посту: оба случая дают ErrNoRows.
func TestComments_InvalidParentRejected(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	author := seedUser(t, repo, "pAuthor", "pa@example.com")
	commenter := seedUser(t, repo, "pCommenter", "pc@example.com")
	postID := seedPost(t, pool, author.ID, "Пост")
	otherPost := seedPost(t, pool, author.ID, "Другой пост")

	root, err := repo.CreateCommentWithAuthor(context.Background(), postID, commenter.ID, "корень", nil)
	if err != nil {
		t.Fatalf("CreateCommentWithAuthor root: %v", err)
	}
	reply, err := repo.CreateCommentWithAuthor(context.Background(), postID, author.ID, "ответ", &root.ID)
	if err != nil {
		t.Fatalf("CreateCommentWithAuthor reply: %v", err)
	}

	// Ответ на ответ запрещён (родитель должен быть корневым).
	if _, err := repo.CreateCommentWithAuthor(context.Background(), postID, author.ID, "ответ на ответ", &reply.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("ответ на ответ: ожидался ErrNoRows, получено %v", err)
	}
	// Родитель из другого поста не принимается.
	if _, err := repo.CreateCommentWithAuthor(context.Background(), otherPost, commenter.ID, "кросс-пост", &root.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("родитель из чужого поста: ожидался ErrNoRows, получено %v", err)
	}
	// Несуществующий пост при parentID=nil: CTE всё равно выполняет INSERT,
	// поэтому падаем на FK (pgcode 23503), а не на ErrNoRows. Это ровно то
	// поведение, которое хендлер трактует как 404 (см. isForeignKeyViolation).
	if _, err := repo.CreateCommentWithAuthor(context.Background(), 999999, commenter.ID, "x", nil); !isFK(err) {
		t.Fatalf("несуществующий пост: ожидалось нарушение FK (23503), получено %v", err)
	}
}

// TestCommentLikes_Idempotent проверяет идемпотентность и корректировку счётчика
// при лайке/анлайке комментария.
func TestCommentLikes_Idempotent(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	author := seedUser(t, repo, "clAuthor", "cla@example.com")
	commenter := seedUser(t, repo, "clCommenter", "clc@example.com")
	postID := seedPost(t, pool, author.ID, "Пост")
	root, err := repo.CreateCommentWithAuthor(context.Background(), postID, commenter.ID, "корень", nil)
	if err != nil {
		t.Fatalf("CreateCommentWithAuthor: %v", err)
	}

	if c, err := repo.AddCommentLike(context.Background(), root.ID, commenter.ID); err != nil || c != 1 {
		t.Fatalf("AddCommentLike: c=%d err=%v", c, err)
	}
	if c, err := repo.AddCommentLike(context.Background(), root.ID, commenter.ID); err != nil || c != 1 {
		t.Fatalf("повторный AddCommentLike: c=%d err=%v", c, err)
	}
	if c, err := repo.RemoveCommentLike(context.Background(), root.ID, commenter.ID); err != nil || c != 0 {
		t.Fatalf("RemoveCommentLike: c=%d err=%v", c, err)
	}
	if c, err := repo.RemoveCommentLike(context.Background(), root.ID, commenter.ID); err != nil || c != 0 {
		t.Fatalf("повторный RemoveCommentLike: c=%d err=%v", c, err)
	}
}

// TestComment_DeletePermissions проверяет права удаления: автор комментария и
// автор поста могут удалить, посторонний — нет.
func TestComment_DeletePermissions(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	postAuthor := seedUser(t, repo, "dpPost", "dpp@example.com")
	commenter := seedUser(t, repo, "dpCommenter", "dpc@example.com")
	stranger := seedUser(t, repo, "dpStranger", "dps@example.com")
	postID := seedPost(t, pool, postAuthor.ID, "Пост")

	c1, err := repo.CreateCommentWithAuthor(context.Background(), postID, commenter.ID, "к1", nil)
	if err != nil {
		t.Fatalf("CreateCommentWithAuthor: %v", err)
	}
	if ok, err := repo.DeleteCommentByID(context.Background(), c1.ID, stranger.ID); err != nil || ok {
		t.Fatalf("посторонний не должен удалять: ok=%v err=%v", ok, err)
	}
	if ok, err := repo.DeleteCommentByID(context.Background(), c1.ID, commenter.ID); err != nil || !ok {
		t.Fatalf("автор комментария должен удалять: ok=%v err=%v", ok, err)
	}

	c2, err := repo.CreateCommentWithAuthor(context.Background(), postID, commenter.ID, "к2", nil)
	if err != nil {
		t.Fatalf("CreateCommentWithAuthor: %v", err)
	}
	if ok, err := repo.DeleteCommentByID(context.Background(), c2.ID, postAuthor.ID); err != nil || !ok {
		t.Fatalf("автор поста должен удалять чужой комментарий: ok=%v err=%v", ok, err)
	}
}

// TestComment_DeleteCascadeReplies проверяет каскадное удаление ответов вместе
// с корневым комментарием.
func TestComment_DeleteCascadeReplies(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	author := seedUser(t, repo, "cascAuthor", "casca@example.com")
	commenter := seedUser(t, repo, "cascCommenter", "cascc@example.com")
	postID := seedPost(t, pool, author.ID, "Пост")

	root, err := repo.CreateCommentWithAuthor(context.Background(), postID, commenter.ID, "корень", nil)
	if err != nil {
		t.Fatalf("CreateCommentWithAuthor: %v", err)
	}
	if _, err := repo.CreateCommentWithAuthor(context.Background(), postID, author.ID, "ответ", &root.ID); err != nil {
		t.Fatalf("CreateCommentWithAuthor reply: %v", err)
	}

	if ok, err := repo.DeleteCommentByID(context.Background(), root.ID, commenter.ID); err != nil || !ok {
		t.Fatalf("удаление корня: ok=%v err=%v", ok, err)
	}
	if _, err := repo.GetCommentByID(context.Background(), root.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("корень должен быть удалён: %v", err)
	}
	tree, err := repo.GetCommentsByPostID(context.Background(), postID, 50, 0, 0)
	if err != nil {
		t.Fatalf("GetCommentsByPostID: %v", err)
	}
	if len(tree) != 0 {
		t.Fatalf("после каскада ожидался пустой список, получено %d", len(tree))
	}
}

// isFK — true, если ошибка является нарушением внешнего ключа PostgreSQL
// (pgcode 23503). Дублирует api.isForeignKeyViolation, но локально в пакете
// repository, чтобы интеграционный тест не зависел от внутренностей api.
func isFK(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
