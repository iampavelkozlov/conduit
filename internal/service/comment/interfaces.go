package comment

import (
	"context"

	"conduit/internal/gen/postgres"
	"conduit/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

//go:generate go run go.uber.org/mock/mockgen -source=interfaces.go -destination=mock_comment.go -package=comment
type repository interface {
	commentRepository
	GetArticleIDBySlug(context.Context, string) (pgtype.UUID, error)
}

type commentRepository interface {
	CreateComment(context.Context, postgres.CreateCommentParams) (postgres.CreateCommentRow, error)
	GetCommentAuthorIDByIDAndArticleID(context.Context, postgres.GetCommentAuthorIDByIDAndArticleIDParams) (pgtype.UUID, error)
	ListCommentsByArticleID(context.Context, pgtype.UUID) ([]postgres.ListCommentsByArticleIDRow, error)
	DeleteCommentByIDAndArticleIDAndAuthorID(context.Context, postgres.DeleteCommentByIDAndArticleIDAndAuthorIDParams) (int64, error)
}

type ProfileReader interface {
	ProfilesByIDs(context.Context, []uuid.UUID) (map[uuid.UUID]models.Profile, error)
}

// UserService is kept as the monolith-facing compatibility boundary while
// ProfileReader names the same narrow capability for extracted transports.
type UserService interface {
	ProfileReader
}

type ArticleResolver interface {
	ResolveArticleID(context.Context, string) (uuid.UUID, error)
}
