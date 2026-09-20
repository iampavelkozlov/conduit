package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	api "conduit/internal/gen/http"
	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type updateFieldsKey struct{}

// TrackUpdateFields preserves the difference between an omitted field and an
// explicit JSON null, which generated pointer fields cannot carry.
func TrackUpdateFields(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || (r.URL.Path != "/api/user" && !strings.HasPrefix(r.URL.Path, "/api/articles/")) {
			next.ServeHTTP(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				writeErrorResponse(w, http.StatusRequestEntityTooLarge, "body", "request body is too large")
			} else {
				writeErrorResponse(w, http.StatusBadRequest, "body", "invalid request body")
			}
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		var payload map[string]map[string]json.RawMessage
		if json.Unmarshal(body, &payload) == nil {
			r = r.WithContext(context.WithValue(r.Context(), updateFieldsKey{}, payload))
		}
		next.ServeHTTP(w, r)
	})
}

func updateFieldWasSet(ctx context.Context, object, name string) bool {
	payload, _ := ctx.Value(updateFieldsKey{}).(map[string]map[string]json.RawMessage)
	fields := payload[object]
	_, ok := fields[name]
	return ok
}

type Server struct {
	svc applicationService
}

func NewServer(svc ApplicationService) api.StrictServerInterface {
	return Server{svc: svc}
}

// NewResponseErrorHandler converts service errors to the canonical envelope
// without exposing internal details to clients.
func NewResponseErrorHandler() func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, _ *http.Request, err error) {
		status, field, message := http.StatusInternalServerError, "body", "internal server error"
		if apiErr, ok := errors.AsType[*shared.APIError](err); ok {
			status, field, message = apiErr.Status, apiErr.Field, apiErr.Message
		} else {
			switch {
			case isUniqueViolation(err):
				status, field, message = http.StatusConflict, uniqueField(err), "has already been taken"
			case errors.Is(err, shared.ErrUnauthorized):
				status, message = http.StatusUnauthorized, "authentication required"
			case errors.Is(err, shared.ErrForbidden):
				status, message = http.StatusForbidden, "forbidden"
			case errors.Is(err, shared.ErrNotFound), errors.Is(err, pgx.ErrNoRows):
				status, message = http.StatusNotFound, "not found"
			case errors.Is(err, shared.ErrValidation):
				status, message = http.StatusUnprocessableEntity, "validation failed"
			}
		}

		writeErrorResponse(w, status, field, message)
	}
}

func isUniqueViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23505"
}

func uniqueField(err error) string {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok {
		return "body"
	}
	switch pgErr.ConstraintName {
	case "users_username_key":
		return "username"
	case "users_email_key":
		return "email"
	default:
		return "body"
	}
}

func profileResponse(profile models.Profile) api.ProfileResponseJSONResponse {
	return api.ProfileResponseJSONResponse{Profile: profileDTO(profile)}
}

func profileDTO(profile models.Profile) api.Profile {
	return api.Profile{Bio: nullableString(profile.Bio), Following: profile.Following, Image: nullableString(profile.Image), Username: profile.Username}
}

func userResponse(user *models.User) api.UserResponseJSONResponse {
	return api.UserResponseJSONResponse{User: api.User{Bio: nullableString(user.Bio), Email: user.Email, Image: nullableString(user.Image), Token: user.Token, Username: user.Username}}
}

func articleDTO(article *models.Article) api.Article {
	return api.Article{
		Author: profileDTO(article.Author), Body: article.Body, CreatedAt: article.CreatedAt,
		Description: article.Description, Favorited: article.Favorited, FavoritesCount: article.FavoritesCount,
		Slug: article.Slug, TagList: article.TagList, Title: article.Title, UpdatedAt: article.UpdatedAt,
	}
}

func commentDTO(comment *models.Comment) api.Comment {
	return api.Comment{Author: profileDTO(comment.Author), Body: comment.Body, CreatedAt: comment.CreatedAt, Id: comment.ID, UpdatedAt: comment.UpdatedAt}
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func writeErrorResponse(w http.ResponseWriter, status int, field, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.GenericErrorModel{Errors: map[string][]string{field: {message}}})
}
