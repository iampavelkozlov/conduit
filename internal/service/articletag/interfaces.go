package articletag

import (
	"context"

	"conduit/internal/gen/postgres"

	"github.com/jackc/pgx/v5/pgtype"
)

//go:generate go run go.uber.org/mock/mockgen -source=interfaces.go -destination=mock_articletag.go -package=articletag
type repository interface {
	ListArticleIDsByTagID(context.Context, pgtype.UUID) ([]pgtype.UUID, error)
	ListArticleTagRelationsByArticleIDs(context.Context, []pgtype.UUID) ([]postgres.ListArticleTagRelationsByArticleIDsRow, error)
}
