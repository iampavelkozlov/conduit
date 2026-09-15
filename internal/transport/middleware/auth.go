package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"conduit/internal/service"
	"conduit/internal/service/shared"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/google/uuid"
)

type AuthMiddleware struct {
	svc tokenValidator
}

func NewAuthMiddleware(svc *service.Service) *AuthMiddleware {
	return &AuthMiddleware{svc: svc}
}

func (m *AuthMiddleware) Authenticate(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
	if input.SecuritySchemeName != "Token" {
		return nil
	}

	auth := input.RequestValidationInput.Request.Header.Get("Authorization")
	if auth == "" {
		return errors.New("missing Authorization header")
	}

	tokenString, ok := authorizationToken(auth)
	if !ok {
		return errors.New("invalid Authorization header")
	}

	claims, err := m.svc.ValidateAccessToken(tokenString)
	if err != nil {
		return errors.New("invalid jwt")
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return errors.New("invalid user id in jwt")
	}
	newCtx := shared.WithAccessToken(shared.WithUserID(ctx, userID), tokenString)
	input.RequestValidationInput.Request = input.RequestValidationInput.Request.WithContext(newCtx)
	return nil
}

// OptionalAuth only enriches public requests that carry a valid bearer token.
// Authorization requirements themselves remain the OpenAPI validator's job.
func (m *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			next.ServeHTTP(w, r)
			return
		}
		tokenString, ok := authorizationToken(auth)
		if !ok {
			writeUnauthorized(w, "invalid authorization header")
			return
		}
		claims, err := m.svc.ValidateAccessToken(tokenString)
		if err != nil {
			writeUnauthorized(w, "invalid token")
			return
		}
		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			writeUnauthorized(w, "invalid token")
			return
		}
		next.ServeHTTP(w, r.WithContext(shared.WithAccessToken(shared.WithUserID(r.Context(), userID), tokenString)))
	})
}

func authorizationToken(header string) (string, bool) {
	for _, prefix := range []string{"Bearer ", "Token "} {
		if token, ok := strings.CutPrefix(header, prefix); ok {
			return token, token != ""
		}
	}
	return "", false
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]map[string][]string{"errors": {"token": {message}}})
}
