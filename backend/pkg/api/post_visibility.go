package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"

	"cfu-board/pkg/models"
	"cfu-board/pkg/repository"
)

// loadVisiblePost загружает пост и применяет то же правило видимости, что
// GET /posts/{id}: pending/rejected видит только автор. Чужим отдаём 404,
// чтобы не подтверждать существование скрытой публикации (лайки, комментарии).
func loadVisiblePost(
	w http.ResponseWriter,
	repo *repository.UserRepository,
	ctx context.Context,
	postID, viewerID int,
) (*models.Post, bool) {
	post, err := repo.GetPostByID(ctx, postID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
			return nil, false
		}
		slog.Error("не удалось загрузить публикацию", "post_id", postID, "error", err)
		RespondWithError(w, http.StatusInternalServerError, "Ошибка сервера при загрузке публикации")
		return nil, false
	}
	if !post.IsVisibleToViewer(viewerID) {
		RespondWithError(w, http.StatusNotFound, "Публикация не найдена")
		return nil, false
	}
	return post, true
}
