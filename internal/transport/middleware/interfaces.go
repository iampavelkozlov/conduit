package middleware

import "conduit/internal/service/auth"

//go:generate go run go.uber.org/mock/mockgen -source=interfaces.go -destination=mock_middleware.go -package=middleware
type tokenValidator interface {
	ValidateAccessToken(string) (*auth.TokenClaims, error)
}
