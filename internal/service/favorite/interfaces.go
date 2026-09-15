package favorite

import (
	"context"

	"conduit/internal/gen/postgres"

	"github.com/jackc/pgx/v5/pgtype"
)

//go:generate go run go.uber.org/mock/mockgen -source=interfaces.go -destination=mock_favorite.go -package=favorite
type repository interface {
	FavoriteArticle(context.Context, postgres.FavoriteArticleParams) error
	UnfavoriteArticle(context.Context, postgres.UnfavoriteArticleParams) error
	ListFavoriteArticleIDsByUserID(context.Context, pgtype.UUID) ([]pgtype.UUID, error)
	CountFavoritesByArticleIDs(context.Context, []pgtype.UUID) ([]postgres.CountFavoritesByArticleIDsRow, error)
	ListFavoriteArticleIDsByUserIDAndArticleIDs(context.Context, postgres.ListFavoriteArticleIDsByUserIDAndArticleIDsParams) ([]pgtype.UUID, error)
}
