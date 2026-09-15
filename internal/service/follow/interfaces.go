package follow

import (
	"context"

	"conduit/internal/gen/postgres"

	"github.com/jackc/pgx/v5/pgtype"
)

//go:generate go run go.uber.org/mock/mockgen -source=interfaces.go -destination=mock_follow.go -package=follow
type repository interface {
	FollowUser(context.Context, postgres.FollowUserParams) error
	UnfollowUser(context.Context, postgres.UnfollowUserParams) error
	IsFollowing(context.Context, postgres.IsFollowingParams) (bool, error)
	ListFolloweeIDsByFollowerID(context.Context, pgtype.UUID) ([]pgtype.UUID, error)
	ListFollowingIDs(context.Context, postgres.ListFollowingIDsParams) ([]pgtype.UUID, error)
}
