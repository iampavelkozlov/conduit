package http

import (
	"errors"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTrackUpdateFields(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       io.ReadCloser
		object     string
		field      string
		wantSet    bool
		wantCalled bool
		wantStatus int
	}{
		{name: "skips unrelated request", method: nethttp.MethodPost, path: "/api/user", body: io.NopCloser(strings.NewReader(`{}`)), wantCalled: true, wantStatus: nethttp.StatusOK},
		{name: "tracks explicit null", method: nethttp.MethodPut, path: "/api/user", body: io.NopCloser(strings.NewReader(`{"user":{"bio":null}}`)), object: "user", field: "bio", wantSet: true, wantCalled: true, wantStatus: nethttp.StatusOK},
		{name: "tracks article field", method: nethttp.MethodPut, path: "/api/articles/slug", body: io.NopCloser(strings.NewReader(`{"article":{"title":"new"}}`)), object: "article", field: "title", wantSet: true, wantCalled: true, wantStatus: nethttp.StatusOK},
		{name: "ignores malformed json", method: nethttp.MethodPut, path: "/api/user", body: io.NopCloser(strings.NewReader(`{`)), object: "user", field: "bio", wantCalled: true, wantStatus: nethttp.StatusOK},
		{name: "handles read error", method: nethttp.MethodPut, path: "/api/user", body: errorReadCloser{err: errors.New("read failed")}, wantStatus: nethttp.StatusBadRequest},
		{name: "handles oversized body", method: nethttp.MethodPut, path: "/api/user", body: errorReadCloser{err: &nethttp.MaxBytesError{Limit: 1}}, wantStatus: nethttp.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			fieldWasSet := false
			next := nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
				called = true
				fieldWasSet = updateFieldWasSet(r.Context(), tt.object, tt.field)
				w.WriteHeader(nethttp.StatusOK)
			})
			request := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
			request.Body = tt.body
			recorder := httptest.NewRecorder()

			TrackUpdateFields(next).ServeHTTP(recorder, request)

			require.Equal(t, tt.wantCalled, called)
			if called {
				require.Equal(t, tt.wantSet, fieldWasSet)
			}
			require.Equal(t, tt.wantStatus, recorder.Code)
		})
	}
}

func TestResponseErrorHandler(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantField  string
	}{
		{name: "typed validation", err: shared.Validation("title", "can't be blank"), wantStatus: nethttp.StatusUnprocessableEntity, wantField: "title"},
		{name: "duplicate username", err: &pgconn.PgError{Code: "23505", ConstraintName: "users_username_key"}, wantStatus: nethttp.StatusConflict, wantField: "username"},
		{name: "duplicate email", err: &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}, wantStatus: nethttp.StatusConflict, wantField: "email"},
		{name: "other duplicate", err: &pgconn.PgError{Code: "23505", ConstraintName: "articles_slug_key"}, wantStatus: nethttp.StatusConflict, wantField: "body"},
		{name: "unauthorized", err: shared.ErrUnauthorized, wantStatus: nethttp.StatusUnauthorized, wantField: "body"},
		{name: "forbidden", err: shared.ErrForbidden, wantStatus: nethttp.StatusForbidden, wantField: "body"},
		{name: "not found", err: shared.ErrNotFound, wantStatus: nethttp.StatusNotFound, wantField: "body"},
		{name: "pgx not found", err: pgx.ErrNoRows, wantStatus: nethttp.StatusNotFound, wantField: "body"},
		{name: "validation sentinel", err: shared.ErrValidation, wantStatus: nethttp.StatusUnprocessableEntity, wantField: "body"},
		{name: "unexpected", err: errors.New("boom"), wantStatus: nethttp.StatusInternalServerError, wantField: "body"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), nethttp.MethodGet, "/api/test", nil)

			NewResponseErrorHandler()(recorder, request, tt.err)

			require.Equal(t, tt.wantStatus, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"`+tt.wantField+`"`)
			require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
		})
	}

	require.False(t, isUniqueViolation(errors.New("other")))
	require.Equal(t, "body", uniqueField(errors.New("other")))
}

func TestDTOConversions(t *testing.T) {
	now := time.Now()
	profile := models.Profile{Username: "author", Bio: "bio", Image: "image", Following: true}
	article := models.Article{Author: profile, Body: "body", CreatedAt: now, Description: "description", Favorited: true, FavoritesCount: 2, Slug: "slug", TagList: []string{"go"}, Title: "title", UpdatedAt: now}
	comment := models.Comment{Author: profile, Body: "comment", CreatedAt: now, ID: 7, UpdatedAt: now}
	user := models.User{Bio: "bio", Email: "user@example.com", Image: "image", Token: "token", Username: "user"}

	require.Equal(t, "author", profileResponse(profile).Profile.Username)
	require.Equal(t, "author", profileDTO(profile).Username)
	require.Equal(t, "user", userResponse(&user).User.Username)
	require.Equal(t, "title", articleDTO(&article).Title)
	require.Equal(t, "title", articleListDTO(&article).Title)
	require.Equal(t, 7, commentDTO(&comment).Id)
	require.Nil(t, nullableString(""))
	require.Equal(t, "value", *nullableString("value"))

	mock := NewMockapplicationService(gomock.NewController(t))
	_, ok := NewServer(nil).(Server)
	require.True(t, ok)
	require.Equal(t, mock, Server{svc: mock}.svc)
}

type errorReadCloser struct{ err error }

func (r errorReadCloser) Read([]byte) (int, error) { return 0, r.err }
func (errorReadCloser) Close() error               { return nil }
