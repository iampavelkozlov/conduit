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
	CreateComment(context.Context, postgres.CreateCommentParams) (postgres.CreateCommentRow, error)
	GetArticleIDBySlug(context.Context, string) (pgtype.UUID, error)
	GetCommentAuthorIDByIDAndArticleID(context.Context, postgres.GetCommentAuthorIDByIDAndArticleIDParams) (pgtype.UUID, error)
	ListCommentsByArticleID(context.Context, pgtype.UUID) ([]postgres.ListCommentsByArticleIDRow, error)
	DeleteCommentByIDAndArticleIDAndAuthorID(context.Context, postgres.DeleteCommentByIDAndArticleIDAndAuthorIDParams) (int64, error)
}
type UserService interface {
	ProfilesByIDs(context.Context, []uuid.UUID) (map[uuid.UUID]models.Profile, error)
}
