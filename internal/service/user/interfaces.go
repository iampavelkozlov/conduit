package user

import (
	"context"

	"conduit/internal/gen/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

//go:generate go run go.uber.org/mock/mockgen -source=interfaces.go -destination=mock_user.go -package=user

type repository interface {
	GetUserByID(context.Context, pgtype.UUID) (postgres.User, error)
	GetUserByUsername(context.Context, string) (postgres.User, error)
	GetUserIDByUsername(context.Context, string) (pgtype.UUID, error)
	ListProfilesByIDs(context.Context, []pgtype.UUID) ([]postgres.ListProfilesByIDsRow, error)
	UpdateUser(context.Context, postgres.UpdateUserParams) (postgres.User, error)
}

type FollowService interface {
	Follow(context.Context, uuid.UUID, uuid.UUID) error
	Unfollow(context.Context, uuid.UUID, uuid.UUID) error
	IsFollowing(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	FollowingIDs(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]struct{}, error)
}

type PasswordManager interface{ Hash(string) (string, error) }
