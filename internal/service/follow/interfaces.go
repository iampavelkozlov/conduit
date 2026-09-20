package follow

import (
	"context"

	"conduit/internal/gen/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Dependency is the complete follow capability consumed by the profile and
// posts services. Both the local service and the remote gRPC adapter implement
// it, which lets the composition root switch transports without owning a
// domain interface.
type Dependency interface {
	Follow(context.Context, uuid.UUID, uuid.UUID) error
	Unfollow(context.Context, uuid.UUID, uuid.UUID) error
	IsFollowing(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	FolloweeIDs(context.Context, uuid.UUID) ([]uuid.UUID, error)
	FollowingIDs(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]struct{}, error)
}

//go:generate go run go.uber.org/mock/mockgen -source=interfaces.go -destination=mock_follow.go -package=follow
type repository interface {
	FollowUser(context.Context, postgres.FollowUserParams) error
	UnfollowUser(context.Context, postgres.UnfollowUserParams) error
	IsFollowing(context.Context, postgres.IsFollowingParams) (bool, error)
	ListFolloweeIDsByFollowerID(context.Context, pgtype.UUID) ([]pgtype.UUID, error)
	ListFollowingIDs(context.Context, postgres.ListFollowingIDsParams) ([]pgtype.UUID, error)
}
