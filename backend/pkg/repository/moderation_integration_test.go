package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"cfu-board/pkg/models"
)

// TestModeration_NewPostIsPending проверяет, что новый пост создаётся со
// статусом pending и НЕ виден в общей ленте, пока не одобрен.
func TestModeration_NewPostIsPending(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	author := seedUser(t, repo, "mAuthor", "ma@example.com")
	viewer := seedUser(t, repo, "mViewer", "mv@example.com")

	id, err := repo.CreatePostWithMedia(context.Background(), author.ID, "На модерации", "текст", "", "")
	if err != nil {
		t.Fatalf("CreatePostWithMedia: %v", err)
	}

	// Статус нового поста — pending.
	created, err := repo.GetPostByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetPostByID: %v", err)
	}
	if created.Status != models.PostStatusPending {
		t.Fatalf("новый пост должен быть pending, получено %q", created.Status)
	}

	// В общей ленте его нет ни у анонима, ни у другого пользователя.
	for _, viewerID := range []int{0, viewer.ID} {
		feed, err := repo.ListPostsWithLikes(context.Background(), viewerID, 20, 0)
		if err != nil {
			t.Fatalf("ListPostsWithLikes viewer=%d: %v", viewerID, err)
		}
		if len(feed) != 0 {
			t.Fatalf("pending-пост не должен попадать в общую ленту, найдено %d", len(feed))
		}
	}

	// Очередь модерации содержит ровно этот пост.
	pending, err := repo.ListPendingPosts(context.Background(), 20, 0)
	if err != nil {
		t.Fatalf("ListPendingPosts: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != id {
		t.Fatalf("очередь модерации: %+v", pending)
	}

	// Автор видит свой pending-пост в личной ленте.
	own, err := repo.ListPostsByAuthorWithLikes(context.Background(), author.ID, author.ID, 20, 0)
	if err != nil {
		t.Fatalf("ListPostsByAuthorWithLikes (автор): %v", err)
	}
	if len(own) != 1 || own[0].ID != id {
		t.Fatalf("автор должен видеть свой pending-пост, получено %+v", own)
	}

	// А вот другой пользователь в профиле автора pending-пост НЕ видит.
	other, err := repo.ListPostsByAuthorWithLikes(context.Background(), viewer.ID, author.ID, 20, 0)
	if err != nil {
		t.Fatalf("ListPostsByAuthorWithLikes (чужой): %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("чужой пользователь не должен видеть pending-пост автора, получено %d", len(other))
	}
}

// TestModeration_ApprovePublishes проверяет, что одобрение публикует пост в
// общую ленту, убирает его из очереди и фиксирует модератора.
func TestModeration_ApprovePublishes(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	admin := seedUser(t, repo, "mAdmin", "madmin@example.com")
	author := seedUser(t, repo, "mAuthor2", "ma2@example.com")

	id, err := repo.CreatePostWithMedia(context.Background(), author.ID, "Скоро в ленте", "текст", "", "")
	if err != nil {
		t.Fatalf("CreatePostWithMedia: %v", err)
	}

	ok, err := repo.SetPostModeration(context.Background(), id, models.PostStatusApproved, admin.ID, "")
	if err != nil || !ok {
		t.Fatalf("SetPostModeration(approve): ok=%v err=%v", ok, err)
	}

	// Пост исчез из очереди модерации.
	if n, err := repo.CountPendingPosts(context.Background()); err != nil || n != 0 {
		t.Fatalf("после одобрения очередь должна быть пуста: n=%d err=%v", n, err)
	}

	// И появляется в общей ленте у анонима.
	feed, err := repo.ListPostsWithLikes(context.Background(), 0, 20, 0)
	if err != nil {
		t.Fatalf("ListPostsWithLikes: %v", err)
	}
	if len(feed) != 1 || feed[0].ID != id || feed[0].Status != models.PostStatusApproved {
		t.Fatalf("одобренный пост должен быть в ленте: %+v", feed)
	}

	// Модератор и время решения зафиксированы.
	p, err := repo.GetPostByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetPostByID: %v", err)
	}
	if p.ModeratedBy == nil || *p.ModeratedBy != admin.ID {
		t.Fatalf("ModeratedBy должен быть %d, получено %v", admin.ID, p.ModeratedBy)
	}
	if p.ModeratedAt == nil {
		t.Fatal("ModeratedAt должен быть заполнен")
	}
	if p.RejectionReason != "" {
		t.Fatalf("у одобренного поста не должно быть причины отказа, получено %q", p.RejectionReason)
	}
}

// TestModeration_RejectHidesAndStoresReason проверяет, что отклонение скрывает
// пост из ленты и очереди, сохраняет причину и не делает его публичным.
func TestModeration_RejectHidesAndStoresReason(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	admin := seedUser(t, repo, "mAdmin3", "madmin3@example.com")
	author := seedUser(t, repo, "mAuthor3", "ma3@example.com")

	id, err := repo.CreatePostWithMedia(context.Background(), author.ID, "Отклонённый", "текст", "", "")
	if err != nil {
		t.Fatalf("CreatePostWithMedia: %v", err)
	}
	if ok, err := repo.SetPostModeration(context.Background(), id, models.PostStatusRejected, admin.ID, "спам/реклама"); err != nil || !ok {
		t.Fatalf("SetPostModeration(reject): ok=%v err=%v", ok, err)
	}

	p, err := repo.GetPostByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetPostByID: %v", err)
	}
	if p.Status != models.PostStatusRejected || p.RejectionReason != "спам/реклама" {
		t.Fatalf("отклонённый пост: status=%q reason=%q", p.Status, p.RejectionReason)
	}

	// Не в общей ленте и не в очереди модерации.
	if feed, _ := repo.ListPostsWithLikes(context.Background(), 0, 20, 0); len(feed) != 0 {
		t.Fatalf("отклонённый пост не должен быть в ленте, найдено %d", len(feed))
	}
	if n, _ := repo.CountPendingPosts(context.Background()); n != 0 {
		t.Fatalf("отклонённый пост не должен ждать модерации, n=%d", n)
	}

	// Прямой просмотр: автору — доступно, анониму — скрыт (ErrNoRows).
	if _, err := repo.GetPostWithLikes(context.Background(), id, author.ID); err != nil {
		t.Fatalf("автор должен видеть свой отклонённый пост: %v", err)
	}
	if _, err := repo.GetPostWithLikes(context.Background(), id, 0); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("аноним не должен видеть отклонённый пост, ожидался ErrNoRows, получено %v", err)
	}
}

// TestModeration_SetUnknownPost проверяет, что решение по несуществующему посту
// возвращает false (не найдено).
func TestModeration_SetUnknownPost(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)
	admin := seedUser(t, repo, "mAdmin4", "madmin4@example.com")

	ok, err := repo.SetPostModeration(context.Background(), 999999, models.PostStatusApproved, admin.ID, "")
	if err != nil || ok {
		t.Fatalf("модерация несуществующего поста: ok=%v err=%v", ok, err)
	}
}
