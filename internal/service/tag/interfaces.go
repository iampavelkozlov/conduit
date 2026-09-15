package tag

import (
	"context"

	"conduit/internal/gen/postgres"

	"github.com/jackc/pgx/v5/pgtype"
)

//go:generate go run go.uber.org/mock/mockgen -source=interfaces.go -destination=mock_tag.go -package=tag
type repository interface {
	GetTagIDByName(context.Context, string) (pgtype.UUID, error)
	ListTags(context.Context) ([]string, error)
	ListTagsByIDs(context.Context, []pgtype.UUID) ([]postgres.ListTagsByIDsRow, error)
}
