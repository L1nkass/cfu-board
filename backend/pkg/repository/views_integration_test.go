package repository

import (
	"context"
	"testing"
)

// TestIncrementPostViews_DedupByIP проверяет дедупликацию просмотров по IP:
// первый визит с IP даёт +1, повтор с того же IP счётчик не меняет,
// визит с другого IP снова даёт +1. Пустой IP не засчитывается.
func TestIncrementPostViews_DedupByIP(t *testing.T) {
	truncateAll(t)
	pool := requirePool(t)
	repo := NewUserRepository(pool)
	ctx := context.Background()

	author := seedUser(t, repo, "viewer", "viewer@example.com")
	postID := seedPost(t, pool, author.ID, "Просматриваемый")

	if v, err := repo.IncrementPostViews(ctx, postID, "203.0.113.10"); err != nil || v != 1 {
		t.Fatalf("первый просмотр: views=%d, err=%v (ожидалось 1, nil)", v, err)
	}
	if v, err := repo.IncrementPostViews(ctx, postID, "203.0.113.10"); err != nil || v != 1 {
		t.Fatalf("повтор с того же IP: views=%d, err=%v (ожидалось 1, nil)", v, err)
	}
	if v, err := repo.IncrementPostViews(ctx, postID, "198.51.100.7"); err != nil || v != 2 {
		t.Fatalf("новый IP: views=%d, err=%v (ожидалось 2, nil)", v, err)
	}
	if v, err := repo.IncrementPostViews(ctx, postID, ""); err != nil || v != -1 {
		t.Fatalf("пустой IP: views=%d, err=%v (ожидалось -1, nil)", v, err)
	}

	post, err := repo.GetPostWithLikes(ctx, postID, 0)
	if err != nil {
		t.Fatalf("GetPostWithLikes: %v", err)
	}
	if post.ViewsCount != 2 {
		t.Fatalf("итоговый ViewsCount=%d (ожидалось 2)", post.ViewsCount)
	}
}
