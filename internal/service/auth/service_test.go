package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"conduit/internal/config"
	"conduit/internal/gen/postgres"
	"conduit/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNew(t *testing.T) {
	service := New(repositoryStub{}, nil, config.AuthConfig{
		JWTSecret:       "test-secret-that-is-at-least-32-bytes",
		PasswordPepper:  "password-pepper-long-enough",
		AccessTokenTTL:  time.Minute,
		RefreshTokenTTL: time.Hour,
	})
	require.NotNil(t, service)
	require.NotNil(t, service.tokenMgr)
	require.NotNil(t, service.passwordMgr)
}

func TestValidateAccessToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	tokenManager := NewMockTokenManagerIface(ctrl)
	claims := &TokenClaims{UserID: uuid.NewString()}
	repoErr := errors.New("token error")
	tests := []struct {
		name    string
		result  *TokenClaims
		err     error
		wantErr error
	}{
		{name: "returns claims", result: claims},
		{name: "propagates error", err: repoErr, wantErr: repoErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenManager.EXPECT().ValidateAccessToken("token").Return(tt.result, tt.err)
			result, err := NewWithDeps(nil, nil, nil, tokenManager, nil).ValidateAccessToken("token")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, claims, result)
		})
	}
}

type repositoryStub struct {
	UserRepository
	SessionRepository
}

// ── helpers ───────────────────────────────────────────────────────────────────

func makeDBUser(id uuid.UUID, email, username string) postgres.User {
	return postgres.User{
		ID:           pgtype.UUID{Bytes: id, Valid: true},
		Email:        email,
		Username:     username,
		PasswordHash: "hashed",
	}
}

// makeTokenPair returns a TokenPair with a valid UUID in AccessClaims.ID so that
// issueTokensForUser can parse it without error.
func newTokenPair() *TokenPair {
	return &TokenPair{
		AccessToken:  "access.token",
		RefreshToken: "refresh.token",
		AccessClaims: TokenClaims{
			UserID:    uuid.NewString(),
			TokenType: TokenTypeAccess,
		},
		RefreshClaims: TokenClaims{
			UserID:    uuid.NewString(),
			TokenType: TokenTypeRefresh,
		},
	}
}

func buildSvc(ctrl *gomock.Controller) (*Service, *MockUserRepository, *MockSessionRepository, *Mocktransactions, *MockTokenManagerIface, *MockPasswordManagerIface) {
	u := NewMockUserRepository(ctrl)
	s := NewMockSessionRepository(ctrl)
	transactions := NewMocktransactions(ctrl)
	tkn := NewMockTokenManagerIface(ctrl)
	pwd := NewMockPasswordManagerIface(ctrl)
	svc := NewWithDeps(u, s, transactions, tkn, pwd)
	return svc, u, s, transactions, tkn, pwd
}

func expectTransaction(transactions *Mocktransactions, users UserRepository, sessions SessionRepository) {
	transactions.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, operation func(postgres.Querier) error) error {
		return operation(authQuerierAdapter{users: users, sessions: sessions})
	})
}

type authQuerierAdapter struct {
	postgres.Querier
	users    UserRepository
	sessions SessionRepository
}

//nolint:gocritic // The generated postgres.Querier contract passes these parameters by value.
func (a authQuerierAdapter) CreateUser(ctx context.Context, params postgres.CreateUserParams) (postgres.User, error) {
	return a.users.CreateUser(ctx, params)
}

func (a authQuerierAdapter) GetUserByEmail(ctx context.Context, email string) (postgres.User, error) {
	return a.users.GetUserByEmail(ctx, email)
}

func (a authQuerierAdapter) GetUserByID(ctx context.Context, id pgtype.UUID) (postgres.User, error) {
	return a.users.GetUserByID(ctx, id)
}

//nolint:gocritic // The generated postgres.Querier contract passes these parameters by value.
func (a authQuerierAdapter) CreateSession(ctx context.Context, params postgres.CreateSessionParams) error {
	return a.sessions.CreateSession(ctx, params)
}

func (a authQuerierAdapter) GetSessionByUserIDAndJWTIDAndRefreshToken(ctx context.Context, params postgres.GetSessionByUserIDAndJWTIDAndRefreshTokenParams) (postgres.Session, error) {
	return a.sessions.GetSessionByUserIDAndJWTIDAndRefreshToken(ctx, params)
}

//nolint:gocritic // The generated postgres.Querier contract passes these parameters by value.
func (a authQuerierAdapter) RotateSession(ctx context.Context, params postgres.RotateSessionParams) (int64, error) {
	return a.sessions.RotateSession(ctx, params)
}

// ── CreateUser ────────────────────────────────────────────────────────────────

func TestCreateUser(t *testing.T) {
	userID := uuid.New()
	dbUser := makeDBUser(userID, "user@example.com", "testuser")

	tests := []struct {
		description    string
		req            *models.NewUserRequest
		setup          func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface)
		transaction    bool
		transactionErr error
		wantErr        bool
	}{
		{
			description: "error: username is blank",
			req:         &models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Password: "secret"}},
			setup:       func(*MockUserRepository, *MockSessionRepository, *MockTokenManagerIface, *MockPasswordManagerIface) {},
			wantErr:     true,
		},
		{
			description: "error: email is blank",
			req:         &models.NewUserRequest{User: models.NewUser{Password: "secret", Username: "testuser"}},
			setup:       func(*MockUserRepository, *MockSessionRepository, *MockTokenManagerIface, *MockPasswordManagerIface) {},
			wantErr:     true,
		},
		{
			description: "error: password is blank",
			req:         &models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Username: "testuser"}},
			setup:       func(*MockUserRepository, *MockSessionRepository, *MockTokenManagerIface, *MockPasswordManagerIface) {},
			wantErr:     true,
		},
		{
			description: "success: user created and tokens issued",
			req:         &models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Password: "secret", Username: "testuser"}},
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pair := newTokenPair()
				pair.AccessClaims.ID = uuid.NewString()
				pwd.EXPECT().Hash("secret").Return("hashed", nil)
				u.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(dbUser, nil)
				tkn.EXPECT().GeneratePair(gomock.Any()).Return(pair, nil)
				s.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(nil)
			},
			transaction: true,
			wantErr:     false,
		},
		{
			description: "error: password hashing fails",
			req:         &models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Password: "secret", Username: "testuser"}},
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pwd.EXPECT().Hash("secret").Return("", errors.New("hash error"))
			},
			wantErr: true,
		},
		{
			description: "error: DB CreateUser fails",
			req:         &models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Password: "secret", Username: "testuser"}},
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pwd.EXPECT().Hash("secret").Return("hashed", nil)
				u.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(postgres.User{}, errors.New("db error"))
			},
			transaction: true,
			wantErr:     true,
		},
		{
			description: "error: token generation fails",
			req:         &models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Password: "secret", Username: "testuser"}},
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pwd.EXPECT().Hash("secret").Return("hashed", nil)
				u.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(dbUser, nil)
				tkn.EXPECT().GeneratePair(gomock.Any()).Return(nil, errors.New("signing error"))
			},
			transaction: true,
			wantErr:     true,
		},
		{
			description: "error: created user has invalid id",
			req:         &models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Password: "secret", Username: "testuser"}},
			setup: func(u *MockUserRepository, _ *MockSessionRepository, _ *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pwd.EXPECT().Hash("secret").Return("hashed", nil)
				u.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(postgres.User{}, nil)
			},
			transaction: true,
			wantErr:     true,
		},
		{
			description: "error: generated access token has invalid jwt id",
			req:         &models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Password: "secret", Username: "testuser"}},
			setup: func(u *MockUserRepository, _ *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pair := newTokenPair()
				pair.AccessClaims.ID = "invalid"
				pwd.EXPECT().Hash("secret").Return("hashed", nil)
				u.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(dbUser, nil)
				tkn.EXPECT().GeneratePair(gomock.Any()).Return(pair, nil)
			},
			transaction: true,
			wantErr:     true,
		},
		{
			description: "error: session creation fails",
			req:         &models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Password: "secret", Username: "testuser"}},
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pair := newTokenPair()
				pair.AccessClaims.ID = uuid.NewString()
				pwd.EXPECT().Hash("secret").Return("hashed", nil)
				u.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(dbUser, nil)
				tkn.EXPECT().GeneratePair(gomock.Any()).Return(pair, nil)
				s.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(errors.New("session error"))
			},
			transaction: true,
			wantErr:     true,
		},
		{
			description: "error: transaction cannot start",
			req:         &models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Password: "secret", Username: "testuser"}},
			setup: func(_ *MockUserRepository, _ *MockSessionRepository, _ *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pwd.EXPECT().Hash("secret").Return("hashed", nil)
			},
			transactionErr: errors.New("transaction error"),
			wantErr:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc, u, s, transactions, tkn, pwd := buildSvc(ctrl)
			tc.setup(u, s, tkn, pwd)
			if tc.transactionErr != nil {
				transactions.EXPECT().WithTx(gomock.Any(), gomock.Any()).Return(tc.transactionErr)
			} else if tc.transaction {
				expectTransaction(transactions, u, s)
			}

			resp, err := svc.CreateUser(t.Context(), *tc.req)

			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.Equal(t, "user@example.com", resp.User.Email)
				require.Equal(t, "testuser", resp.User.Username)
				require.NotEmpty(t, resp.User.Token)
				require.NotEmpty(t, resp.User.RefreshToken)
			}
		})
	}
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLogin(t *testing.T) {
	userID := uuid.New()
	dbUser := makeDBUser(userID, "user@example.com", "testuser")

	tests := []struct {
		description string
		req         *models.LoginUserRequest
		setup       func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface)
		wantErr     bool
	}{
		{
			description: "error: email is blank",
			req:         &models.LoginUserRequest{User: models.LoginUser{Password: "secret"}},
			setup:       func(*MockUserRepository, *MockSessionRepository, *MockTokenManagerIface, *MockPasswordManagerIface) {},
			wantErr:     true,
		},
		{
			description: "error: password is blank",
			req:         &models.LoginUserRequest{User: models.LoginUser{Email: "user@example.com"}},
			setup:       func(*MockUserRepository, *MockSessionRepository, *MockTokenManagerIface, *MockPasswordManagerIface) {},
			wantErr:     true,
		},
		{
			description: "success: valid credentials",
			req:         &models.LoginUserRequest{User: models.LoginUser{Email: "user@example.com", Password: "secret"}},
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pair := newTokenPair()
				pair.AccessClaims.ID = uuid.NewString()
				u.EXPECT().GetUserByEmail(gomock.Any(), "user@example.com").Return(dbUser, nil)
				pwd.EXPECT().Verify("secret", "hashed").Return(true, nil)
				tkn.EXPECT().GeneratePair(gomock.Any()).Return(pair, nil)
				s.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			description: "error: user not found in DB",
			req:         &models.LoginUserRequest{User: models.LoginUser{Email: "missing@example.com", Password: "secret"}},
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				u.EXPECT().GetUserByEmail(gomock.Any(), "missing@example.com").Return(postgres.User{}, errors.New("not found"))
			},
			wantErr: true,
		},
		{
			description: "error: missing user maps to invalid credentials",
			req:         &models.LoginUserRequest{User: models.LoginUser{Email: "missing@example.com", Password: "secret"}},
			setup: func(u *MockUserRepository, _ *MockSessionRepository, _ *MockTokenManagerIface, _ *MockPasswordManagerIface) {
				u.EXPECT().GetUserByEmail(gomock.Any(), "missing@example.com").Return(postgres.User{}, pgx.ErrNoRows)
			},
			wantErr: true,
		},
		{
			description: "error: password verification returns internal error",
			req:         &models.LoginUserRequest{User: models.LoginUser{Email: "user@example.com", Password: "secret"}},
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				u.EXPECT().GetUserByEmail(gomock.Any(), "user@example.com").Return(dbUser, nil)
				pwd.EXPECT().Verify("secret", "hashed").Return(false, errors.New("argon error"))
			},
			wantErr: true,
		},
		{
			description: "error: wrong password",
			req:         &models.LoginUserRequest{User: models.LoginUser{Email: "user@example.com", Password: "wrong"}},
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				u.EXPECT().GetUserByEmail(gomock.Any(), "user@example.com").Return(dbUser, nil)
				pwd.EXPECT().Verify("wrong", "hashed").Return(false, nil)
			},
			wantErr: true,
		},
		{
			description: "error: token generation fails",
			req:         &models.LoginUserRequest{User: models.LoginUser{Email: "user@example.com", Password: "secret"}},
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				u.EXPECT().GetUserByEmail(gomock.Any(), "user@example.com").Return(dbUser, nil)
				pwd.EXPECT().Verify("secret", "hashed").Return(true, nil)
				tkn.EXPECT().GeneratePair(gomock.Any()).Return(nil, errors.New("signing error"))
			},
			wantErr: true,
		},
		{
			description: "error: session persistence fails",
			req:         &models.LoginUserRequest{User: models.LoginUser{Email: "user@example.com", Password: "secret"}},
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pair := newTokenPair()
				pair.AccessClaims.ID = uuid.NewString()
				u.EXPECT().GetUserByEmail(gomock.Any(), "user@example.com").Return(dbUser, nil)
				pwd.EXPECT().Verify("secret", "hashed").Return(true, nil)
				tkn.EXPECT().GeneratePair(gomock.Any()).Return(pair, nil)
				s.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(errors.New("session error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc, u, s, _, tkn, pwd := buildSvc(ctrl)
			tc.setup(u, s, tkn, pwd)

			resp, err := svc.Login(t.Context(), *tc.req)

			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.Equal(t, "user@example.com", resp.User.Email)
				require.NotEmpty(t, resp.User.Token)
				require.NotEmpty(t, resp.User.RefreshToken)
			}
		})
	}
}

// ── RefreshToken ──────────────────────────────────────────────────────────────

func TestRefreshToken(t *testing.T) {
	userID := uuid.New()
	jwtID := uuid.New()
	dbUser := makeDBUser(userID, "user@example.com", "testuser")

	// validClaims builds a well-formed access-token claims struct.
	validClaims := func() *TokenClaims {
		c := &TokenClaims{
			UserID:    userID.String(),
			TokenType: TokenTypeAccess,
		}
		c.ID = jwtID.String()
		return c
	}
	validRefreshClaims := func(subject string) *TokenClaims {
		c := &TokenClaims{UserID: subject, TokenType: TokenTypeRefresh}
		c.ID = uuid.NewString()
		return c
	}

	tests := []struct {
		description  string
		accessToken  string
		refreshToken string
		setup        func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface)
		wantErr      bool
	}{
		{
			description:  "success: valid tokens, session rotated to new pair",
			accessToken:  "access.token",
			refreshToken: "refresh.token",
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pair := newTokenPair()
				pair.AccessClaims.ID = uuid.NewString()
				tkn.EXPECT().ParseIgnoringExpiry("access.token").Return(validClaims(), nil)
				tkn.EXPECT().ValidateRefreshToken("refresh.token").Return(validRefreshClaims(userID.String()), nil)
				s.EXPECT().GetSessionByUserIDAndJWTIDAndRefreshToken(gomock.Any(), gomock.Any()).Return(postgres.Session{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}}, nil)
				u.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).Return(dbUser, nil)
				tkn.EXPECT().GeneratePair(gomock.Any()).Return(pair, nil)
				s.EXPECT().RotateSession(gomock.Any(), gomock.Any()).Return(int64(1), nil)
			},
			wantErr: false,
		},
		{
			description:  "error: access token cannot be parsed",
			accessToken:  "bad.token",
			refreshToken: "refresh.token",
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				tkn.EXPECT().ParseIgnoringExpiry("bad.token").Return(nil, ErrInvalidToken)
			},
			wantErr: true,
		},
		{
			description:  "error: token is a refresh token, not an access token",
			accessToken:  "refresh.used.as.access",
			refreshToken: "refresh.token",
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				c := &TokenClaims{UserID: userID.String(), TokenType: TokenTypeRefresh}
				c.ID = jwtID.String()
				tkn.EXPECT().ParseIgnoringExpiry("refresh.used.as.access").Return(c, nil)
			},
			wantErr: true,
		},
		{
			description:  "error: user ID in claims is not a valid UUID",
			accessToken:  "access.token",
			refreshToken: "refresh.token",
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				c := &TokenClaims{UserID: "not-a-uuid", TokenType: TokenTypeAccess}
				c.ID = jwtID.String()
				tkn.EXPECT().ParseIgnoringExpiry("access.token").Return(c, nil)
			},
			wantErr: true,
		},
		{
			description:  "error: JWT ID in claims is not a valid UUID",
			accessToken:  "access.token",
			refreshToken: "refresh.token",
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				c := &TokenClaims{UserID: userID.String(), TokenType: TokenTypeAccess}
				c.ID = "not-a-uuid"
				tkn.EXPECT().ParseIgnoringExpiry("access.token").Return(c, nil)
			},
			wantErr: true,
		},
		{
			description:  "error: refresh token is invalid or expired",
			accessToken:  "access.token",
			refreshToken: "bad.refresh.token",
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				tkn.EXPECT().ParseIgnoringExpiry("access.token").Return(validClaims(), nil)
				tkn.EXPECT().ValidateRefreshToken("bad.refresh.token").Return(nil, ErrExpiredToken)
			},
			wantErr: true,
		},
		{
			description:  "error: access and refresh token subjects differ",
			accessToken:  "access.token",
			refreshToken: "refresh.token",
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				tkn.EXPECT().ParseIgnoringExpiry("access.token").Return(validClaims(), nil)
				tkn.EXPECT().ValidateRefreshToken("refresh.token").Return(validRefreshClaims(uuid.NewString()), nil)
			},
			wantErr: true,
		},
		{
			description:  "error: session not found in DB",
			accessToken:  "access.token",
			refreshToken: "refresh.token",
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				tkn.EXPECT().ParseIgnoringExpiry("access.token").Return(validClaims(), nil)
				tkn.EXPECT().ValidateRefreshToken("refresh.token").Return(validRefreshClaims(userID.String()), nil)
				s.EXPECT().GetSessionByUserIDAndJWTIDAndRefreshToken(gomock.Any(), gomock.Any()).Return(postgres.Session{}, errors.New("not found"))
			},
			wantErr: true,
		},
		{
			description:  "error: user not found after session validation",
			accessToken:  "access.token",
			refreshToken: "refresh.token",
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				tkn.EXPECT().ParseIgnoringExpiry("access.token").Return(validClaims(), nil)
				tkn.EXPECT().ValidateRefreshToken("refresh.token").Return(validRefreshClaims(userID.String()), nil)
				s.EXPECT().GetSessionByUserIDAndJWTIDAndRefreshToken(gomock.Any(), gomock.Any()).Return(postgres.Session{}, nil)
				u.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).Return(postgres.User{}, errors.New("user deleted"))
			},
			wantErr: true,
		},
		{
			description:  "error: new token pair generation fails",
			accessToken:  "access.token",
			refreshToken: "refresh.token",
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				tkn.EXPECT().ParseIgnoringExpiry("access.token").Return(validClaims(), nil)
				tkn.EXPECT().ValidateRefreshToken("refresh.token").Return(validRefreshClaims(userID.String()), nil)
				s.EXPECT().GetSessionByUserIDAndJWTIDAndRefreshToken(gomock.Any(), gomock.Any()).Return(postgres.Session{}, nil)
				u.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).Return(dbUser, nil)
				tkn.EXPECT().GeneratePair(gomock.Any()).Return(nil, errors.New("signing error"))
			},
			wantErr: true,
		},
		{
			description:  "error: session rotation fails after token generation",
			accessToken:  "access.token",
			refreshToken: "refresh.token",
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pair := newTokenPair()
				pair.AccessClaims.ID = uuid.NewString()
				tkn.EXPECT().ParseIgnoringExpiry("access.token").Return(validClaims(), nil)
				tkn.EXPECT().ValidateRefreshToken("refresh.token").Return(validRefreshClaims(userID.String()), nil)
				s.EXPECT().GetSessionByUserIDAndJWTIDAndRefreshToken(gomock.Any(), gomock.Any()).Return(postgres.Session{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}}, nil)
				u.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).Return(dbUser, nil)
				tkn.EXPECT().GeneratePair(gomock.Any()).Return(pair, nil)
				s.EXPECT().RotateSession(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("db error"))
			},
			wantErr: true,
		},
		{
			description:  "error: session was concurrently rotated",
			accessToken:  "access.token",
			refreshToken: "refresh.token",
			setup: func(u *MockUserRepository, s *MockSessionRepository, tkn *MockTokenManagerIface, pwd *MockPasswordManagerIface) {
				pair := newTokenPair()
				pair.AccessClaims.ID = uuid.NewString()
				tkn.EXPECT().ParseIgnoringExpiry("access.token").Return(validClaims(), nil)
				tkn.EXPECT().ValidateRefreshToken("refresh.token").Return(validRefreshClaims(userID.String()), nil)
				s.EXPECT().GetSessionByUserIDAndJWTIDAndRefreshToken(gomock.Any(), gomock.Any()).Return(postgres.Session{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}}, nil)
				u.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).Return(dbUser, nil)
				tkn.EXPECT().GeneratePair(gomock.Any()).Return(pair, nil)
				s.EXPECT().RotateSession(gomock.Any(), gomock.Any()).Return(int64(0), nil)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc, u, s, _, tkn, pwd := buildSvc(ctrl)
			tc.setup(u, s, tkn, pwd)

			resp, err := svc.RefreshToken(t.Context(), tc.accessToken, tc.refreshToken)

			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.Equal(t, "user@example.com", resp.User.Email)
				require.NotEmpty(t, resp.User.Token)
				require.NotEmpty(t, resp.User.RefreshToken)
			}
		})
	}
}
