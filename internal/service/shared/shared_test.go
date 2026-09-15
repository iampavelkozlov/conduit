package shared

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestAPIErrors(t *testing.T) {
	tests := []struct {
		name    string
		build   func() error
		status  int
		field   string
		message string
		target  error
	}{
		{name: "validation", build: func() error { return Validation("title", "can't be blank") }, status: http.StatusUnprocessableEntity, field: "title", message: "can't be blank", target: ErrValidation},
		{name: "not found", build: func() error { return NotFound("article") }, status: http.StatusNotFound, field: "article", message: "not found", target: ErrNotFound},
		{name: "forbidden", build: func() error { return Forbidden("comment") }, status: http.StatusForbidden, field: "comment", message: "forbidden", target: ErrForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.build()
			apiErr, ok := errors.AsType[*APIError](err)
			require.True(t, ok)
			require.Equal(t, tt.status, apiErr.Status)
			require.Equal(t, tt.field, apiErr.Field)
			require.Equal(t, tt.message, apiErr.Message)
			require.Equal(t, tt.field+": "+tt.message, err.Error())
			require.ErrorIs(t, err, tt.target)
		})
	}

	unknown := &APIError{Status: http.StatusTeapot, Field: "body", Message: "teapot"}
	require.NoError(t, unknown.Unwrap())
}

func TestContextValues(t *testing.T) {
	userID := uuid.New()
	ctx := WithAccessToken(WithUserID(t.Context(), userID), "token")

	gotUserID, err := UserIDFromContext(ctx)
	require.NoError(t, err)
	require.Equal(t, userID, gotUserID)
	require.Equal(t, "token", AccessTokenFromContext(ctx))

	_, err = UserIDFromContext(t.Context())
	require.ErrorIs(t, err, ErrUserNotFoundInContext)
	require.Empty(t, AccessTokenFromContext(t.Context()))
}

func TestUUIDConversions(t *testing.T) {
	id := uuid.New()
	got, err := PGToUUID(UUIDToPG(id))
	require.NoError(t, err)
	require.Equal(t, id, got)

	_, err = PGToUUID(pgtype.UUID{})
	require.ErrorIs(t, err, ErrInvalidUUID)
}

func TestOptionalTextConversions(t *testing.T) {
	value := "value"

	require.Equal(t, pgtype.Text{String: value, Valid: true}, TextFromPtr(&value))
	require.Equal(t, pgtype.Text{}, TextFromPtr(nil))
	require.Equal(t, value, StringFromPtr(&value))
	require.Empty(t, StringFromPtr(nil))
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name   string
		title  string
		prefix string
	}{
		{name: "normal title", title: "Modern Go Article", prefix: "modern-go-article-"},
		{name: "empty normalized title", title: "!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := GenerateSlug(tt.title)
			if tt.prefix != "" {
				require.True(t, strings.HasPrefix(value, tt.prefix))
				require.Len(t, strings.TrimPrefix(value, tt.prefix), 8)
				return
			}
			_, err := uuid.Parse(value)
			require.NoError(t, err)
		})
	}
}
