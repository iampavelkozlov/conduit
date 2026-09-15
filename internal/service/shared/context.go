package shared

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type contextKey string

const userContextKey contextKey = "user"
const tokenContextKey contextKey = "token"

var ErrUserNotFoundInContext = errors.New("user not found in context")

func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userContextKey, userID)
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	userID, ok := ctx.Value(userContextKey).(uuid.UUID)
	if !ok {
		return uuid.UUID{}, ErrUserNotFoundInContext
	}
	return userID, nil
}

func WithAccessToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenContextKey, token)
}

func AccessTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(tokenContextKey).(string)
	return token
}
