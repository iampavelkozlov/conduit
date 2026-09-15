package auth

import (
	"context"

	"conduit/internal/gen/postgres"

	"github.com/jackc/pgx/v5/pgtype"
)

//go:generate go run go.uber.org/mock/mockgen -destination=mock_auth.go -package=auth conduit/internal/service/auth UserRepository,SessionRepository,TokenManagerIface,PasswordManagerIface

// UserRepository is the DB interface for user operations used by the auth service.
type UserRepository interface {
	CreateUser(ctx context.Context, arg postgres.CreateUserParams) (postgres.User, error)
	GetUserByEmail(ctx context.Context, email string) (postgres.User, error)
	GetUserByID(ctx context.Context, id pgtype.UUID) (postgres.User, error)
}

// SessionRepository is the DB interface for session operations used by the auth service.
type SessionRepository interface {
	CreateSession(ctx context.Context, arg postgres.CreateSessionParams) error
	GetSessionByUserIDAndJWTIDAndRefreshToken(ctx context.Context, arg postgres.GetSessionByUserIDAndJWTIDAndRefreshTokenParams) (postgres.Session, error)
	RotateSession(ctx context.Context, arg postgres.RotateSessionParams) (int64, error)
}

// Repository is the complete persistence contract required in production.
type Repository interface {
	UserRepository
	SessionRepository
}

// TokenManagerIface generates and validates JWT tokens.
type TokenManagerIface interface {
	GeneratePair(userID string) (*TokenPair, error)
	ParseIgnoringExpiry(tokenString string) (*TokenClaims, error)
	ValidateAccessToken(token string) (*TokenClaims, error)
	ValidateRefreshToken(token string) (*TokenClaims, error)
}

// PasswordManagerIface hashes and verifies passwords.
type PasswordManagerIface interface {
	Hash(password string) (string, error)
	Verify(password, encodedHash string) (bool, error)
}
