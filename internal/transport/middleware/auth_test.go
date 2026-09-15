package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"conduit/internal/service/auth"
	"conduit/internal/service/shared"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAuthenticate(t *testing.T) {
	userID := uuid.New()
	serviceErr := errors.New("invalid token")
	tests := []struct {
		name    string
		scheme  string
		header  string
		setup   func(*MocktokenValidator)
		wantErr string
		wantID  uuid.UUID
	}{
		{name: "ignores other schemes", scheme: "Other", setup: func(*MocktokenValidator) {}},
		{name: "requires header", scheme: "Token", setup: func(*MocktokenValidator) {}, wantErr: "missing Authorization header"},
		{name: "rejects malformed header", scheme: "Token", header: "Basic value", setup: func(*MocktokenValidator) {}, wantErr: "invalid Authorization header"},
		{name: "rejects empty token", scheme: "Token", header: "Bearer ", setup: func(*MocktokenValidator) {}, wantErr: "invalid Authorization header"},
		{name: "maps validation error", scheme: "Token", header: "Bearer token", setup: func(mock *MocktokenValidator) {
			mock.EXPECT().ValidateAccessToken("token").Return(nil, serviceErr)
		}, wantErr: "invalid jwt"},
		{name: "rejects invalid user id", scheme: "Token", header: "Token token", setup: func(mock *MocktokenValidator) {
			mock.EXPECT().ValidateAccessToken("token").Return(&auth.TokenClaims{UserID: "invalid"}, nil)
		}, wantErr: "invalid user id in jwt"},
		{name: "enriches request context", scheme: "Token", header: "Bearer token", setup: func(mock *MocktokenValidator) {
			mock.EXPECT().ValidateAccessToken("token").Return(&auth.TokenClaims{UserID: userID.String()}, nil)
		}, wantID: userID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMocktokenValidator(gomock.NewController(t))
			tt.setup(mock)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/test", nil)
			request.Header.Set("Authorization", tt.header)
			input := &openapi3filter.AuthenticationInput{
				SecuritySchemeName: tt.scheme,
				RequestValidationInput: &openapi3filter.RequestValidationInput{
					Request: request,
				},
			}

			err := (&AuthMiddleware{svc: mock}).Authenticate(t.Context(), input)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.wantID != uuid.Nil {
				gotID, idErr := shared.UserIDFromContext(input.RequestValidationInput.Request.Context())
				require.NoError(t, idErr)
				require.Equal(t, tt.wantID, gotID)
				require.Equal(t, "token", shared.AccessTokenFromContext(input.RequestValidationInput.Request.Context()))
			}
		})
	}
}

func TestOptionalAuth(t *testing.T) {
	userID := uuid.New()
	serviceErr := errors.New("invalid token")
	tests := []struct {
		name       string
		header     string
		setup      func(*MocktokenValidator)
		wantCalled bool
		wantStatus int
		wantID     uuid.UUID
	}{
		{name: "allows anonymous request", setup: func(*MocktokenValidator) {}, wantCalled: true, wantStatus: http.StatusNoContent},
		{name: "rejects malformed header", header: "Basic token", setup: func(*MocktokenValidator) {}, wantStatus: http.StatusUnauthorized},
		{name: "rejects invalid token", header: "Bearer token", setup: func(mock *MocktokenValidator) {
			mock.EXPECT().ValidateAccessToken("token").Return(nil, serviceErr)
		}, wantStatus: http.StatusUnauthorized},
		{name: "rejects invalid user id", header: "Bearer token", setup: func(mock *MocktokenValidator) {
			mock.EXPECT().ValidateAccessToken("token").Return(&auth.TokenClaims{UserID: "invalid"}, nil)
		}, wantStatus: http.StatusUnauthorized},
		{name: "enriches authenticated request", header: "Token token", setup: func(mock *MocktokenValidator) {
			mock.EXPECT().ValidateAccessToken("token").Return(&auth.TokenClaims{UserID: userID.String()}, nil)
		}, wantCalled: true, wantStatus: http.StatusNoContent, wantID: userID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMocktokenValidator(gomock.NewController(t))
			tt.setup(mock)
			called := false
			var (
				gotID      uuid.UUID
				contextErr error
				gotToken   string
			)
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if tt.wantID != uuid.Nil {
					gotID, contextErr = shared.UserIDFromContext(r.Context())
					gotToken = shared.AccessTokenFromContext(r.Context())
				}
				w.WriteHeader(http.StatusNoContent)
			})
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/articles", nil)
			request.Header.Set("Authorization", tt.header)
			recorder := httptest.NewRecorder()

			(&AuthMiddleware{svc: mock}).OptionalAuth(next).ServeHTTP(recorder, request)

			require.Equal(t, tt.wantCalled, called)
			if tt.wantID != uuid.Nil {
				require.NoError(t, contextErr)
				require.Equal(t, tt.wantID, gotID)
				require.Equal(t, "token", gotToken)
			}
			require.Equal(t, tt.wantStatus, recorder.Code)
			if tt.wantStatus == http.StatusUnauthorized {
				require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
				require.Contains(t, recorder.Body.String(), `"token"`)
			}
		})
	}
}

func TestAuthorizationToken(t *testing.T) {
	tests := []struct {
		header string
		token  string
		ok     bool
	}{
		{header: "Bearer value", token: "value", ok: true},
		{header: "Token value", token: "value", ok: true},
		{header: "Bearer "},
		{header: "Basic value"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			token, ok := authorizationToken(tt.header)
			require.Equal(t, tt.token, token)
			require.Equal(t, tt.ok, ok)
		})
	}
}

func TestNewAuthMiddleware(t *testing.T) {
	require.NotNil(t, NewAuthMiddleware(nil))
}

func TestJSONHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	JSONHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	require.Equal(t, http.StatusNoContent, recorder.Code)
}
