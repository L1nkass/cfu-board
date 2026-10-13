package repository

import (
	"context"
	"testing"

	"cfu-board/pkg/models"
)

// TestPostRepository_CreateGetDelete проверяет создание поста, чтение по ID,
// маппинг пустых медиа-URL и правила удаления: чужой пост удалить нельзя, свой
// — можно, повторное удаление возвращает false.
func TestPostRepository_CreateGetDelete(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	author := seedUser(t, repo, "author", "author@example.com")
	other := seedUser(t, repo, "other", "other@example.com")
	postID := seedPost(t, pool, author.ID, "Мой пост")

	got, err := repo.GetPostByID(context.Background(), postID)
	if err != nil {
		t.Fatalf("GetPostByID: %v", err)
	}
	if got.Title != "Мой пост" || got.AuthorID != author.ID {
		t.Fatalf("GetPostByID вернул неожиданные данные: %+v", got)
	}
	if got.ImageURL != "" || got.VideoURL != "" {
		t.Fatalf("медиа должны быть пустыми строками: %+v", got)
	}

	if ok, err := repo.DeletePostByID(context.Background(), postID, other.ID); err != nil || ok {
		t.Fatalf("чужой пост не должен удаляться: ok=%v err=%v", ok, err)
	}
	if ok, err := repo.DeletePostByID(context.Background(), postID, author.ID); err != nil || !ok {
		t.Fatalf("свой пост должен удаляться: ok=%v err=%v", ok, err)
	}
	if ok, err := repo.DeletePostByID(context.Background(), postID, author.ID); err != nil || ok {
		t.Fatalf("повторное удаление: ok=%v err=%v", ok, err)
	}
}

// TestPostRepository_CreateWithMedia проверяет сохранение и чтение медиа-URL.
func TestPostRepository_CreateWithMedia(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)
	author := seedUser(t, repo, "media", "media@example.com")

	postID, err := repo.CreatePostWithMedia(context.Background(), author.ID, "С медиа", "текст",
		"http://img/1.png", "http://vid/1.mp4")
	if err != nil {
		t.Fatalf("CreatePostWithMedia: %v", err)
	}
	// Новый пост создаётся со статусом pending (на модерации).
	if pending, err := repo.GetPostByID(context.Background(), postID); err != nil || pending.Status != models.PostStatusPending {
		t.Fatalf("новый пост должен быть pending, получено status=%q err=%v", pending.Status, err)
	}
	got, err := repo.GetPostByID(context.Background(), postID)
	if err != nil {
		t.Fatalf("GetPostByID: %v", err)
	}
	if got.ImageURL != "http://img/1.png" || got.VideoURL != "http://vid/1.mp4" {
		t.Fatalf("медиа не сохранились: %+v", got)
	}
}

// TestPostRepository_CountAndListByAuthor проверяет счётчик постов автора,
// порядок ленты (новые сверху) и пагинацию limit/offset.
func TestPostRepository_CountAndListByAuthor(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	a := seedUser(t, repo, "feedA", "a@example.com")
	b := seedUser(t, repo, "feedB", "b@example.com")

	id1 := seedPost(t, pool, a.ID, "A1")
	id2 := seedPost(t, pool, a.ID, "A2")
	id3 := seedPost(t, pool, a.ID, "A3")
	seedPost(t, pool, b.ID, "B1")

	count, err := repo.CountPostsByAuthor(context.Background(), a.ID, 0)
	if err != nil || count != 3 {
		t.Fatalf("CountPostsByAuthor A: count=%d err=%v", count, err)
	}

	posts, err := repo.ListPostsByAuthorWithLikes(context.Background(), 0, a.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListPostsByAuthorWithLikes: %v", err)
	}
	if len(posts) != 3 {
		t.Fatalf("ожидалось 3 поста у автора, получено %d", len(posts))
	}
	// Новые сверху: id3 > id2 > id1.
	if posts[0].ID != id3 || posts[1].ID != id2 || posts[2].ID != id1 {
		t.Fatalf("неверный порядок ленты: %d %d %d", posts[0].ID, posts[1].ID, posts[2].ID)
	}

	// Пагинация: вторая страница по одному посту.
	page, err := repo.ListPostsByAuthorWithLikes(context.Background(), 0, a.ID, 1, 1)
	if err != nil {
		t.Fatalf("пагинация: %v", err)
	}
	if len(page) != 1 || page[0].ID != id2 {
		t.Fatalf("пагинация вернула неожиданный результат: %+v", page)
	}
}

// TestPostRepository_FeedLikesAndViewer проверяет ленту с лайками: liked_by_me
// для анонима и автора лайка, идемпотентность Add/RemoveLike и счётчик лайков.
func TestPostRepository_FeedLikesAndViewer(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)

	author := seedUser(t, repo, "vAuthor", "va@example.com")
	viewer := seedUser(t, repo, "vViewer", "vv@example.com")
	postID := seedPost(t, pool, author.ID, "Лайкнутый")

	anon, err := repo.GetPostWithLikes(context.Background(), postID, -1)
	if err != nil {
		t.Fatalf("GetPostWithLikes anon: %v", err)
	}
	if anon.LikesCount != 0 || anon.LikedByMe {
		t.Fatalf("anon: %+v", anon)
	}
	if anon.AuthorName != "vAuthor" {
		t.Fatalf("AuthorName: %q", anon.AuthorName)
	}

	if c, added, err := repo.AddLike(context.Background(), postID, viewer.ID); err != nil || c != 1 || !added {
		t.Fatalf("AddLike: c=%d added=%v err=%v", c, added, err)
	}
	// Повторный лайк идемпотентен — счётчик не растёт.
	if c, added, err := repo.AddLike(context.Background(), postID, viewer.ID); err != nil || c != 1 || added {
		t.Fatalf("повторный AddLike: c=%d added=%v err=%v", c, added, err)
	}

	asViewer, err := repo.GetPostWithLikes(context.Background(), postID, viewer.ID)
	if err != nil {
		t.Fatalf("GetPostWithLikes viewer: %v", err)
	}
	if asViewer.LikesCount != 1 || !asViewer.LikedByMe {
		t.Fatalf("viewer: %+v", asViewer)
	}

	if c, err := repo.RemoveLike(context.Background(), postID, viewer.ID); err != nil || c != 0 {
		t.Fatalf("RemoveLike: c=%d err=%v", c, err)
	}
	if c, err := repo.RemoveLike(context.Background(), postID, viewer.ID); err != nil || c != 0 {
		t.Fatalf("повторный RemoveLike: c=%d err=%v", c, err)
	}
}

// TestAddLike_MissingPostFK проверяет, что лайк несуществующего поста приводит
// к ошибке FK (23503), которую хендлер трактует как 404.
func TestAddLike_MissingPostFK(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)
	u := seedUser(t, repo, "liker", "liker@example.com")

	if _, _, err := repo.AddLike(context.Background(), 999999, u.ID); err == nil {
		t.Fatal("лайк несуществующего поста должен вернуть ошибку FK")
	}
}
