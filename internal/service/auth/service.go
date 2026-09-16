package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"conduit/internal/config"
	"conduit/internal/gen/postgres"
	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	userRepo     UserRepository
	sessionRepo  SessionRepository
	transactions transactions
	tokenMgr     TokenManagerIface
	passwordMgr  PasswordManagerIface
	logger       *slog.Logger
}

func New(repo Repository, transactions transactions, cfg config.AuthConfig, loggers ...*slog.Logger) *Service {
	return &Service{
		userRepo:     repo,
		sessionRepo:  repo,
		transactions: transactions,
		tokenMgr:     NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL),
		passwordMgr:  NewPasswordManager(cfg.PasswordPepper),
		logger:       shared.ServiceLogger(loggers...),
	}
}

// NewWithDeps constructs a Service from explicit interface dependencies.
// Intended for use in tests.
func NewWithDeps(
	userRepo UserRepository,
	sessionRepo SessionRepository,
	transactions transactions,
	tokenMgr TokenManagerIface,
	passwordMgr PasswordManagerIface,
	loggers ...*slog.Logger,
) *Service {
	return &Service{
		userRepo:     userRepo,
		sessionRepo:  sessionRepo,
		transactions: transactions,
		tokenMgr:     tokenMgr,
		passwordMgr:  passwordMgr,
		logger:       shared.ServiceLogger(loggers...),
	}
}

func (s *Service) CreateUser(ctx context.Context, req models.NewUserRequest) (*models.UserResponse, error) {
	if req.User.Username == "" {
		return nil, shared.Validation("username", "can't be blank")
	}
	if req.User.Email == "" {
		return nil, shared.Validation("email", "can't be blank")
	}
	if req.User.Password == "" {
		return nil, shared.Validation("password", "can't be blank")
	}
	passwordHash, err := s.passwordMgr.Hash(req.User.Password)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "auth password err", shared.ErrorAttrs(err)...)
		return nil, fmt.Errorf("hash password: %w", err)
	}

	userID := shared.NewUUIDValue()
	var response *models.UserResponse
	err = s.transactions.WithTx(ctx, func(repo postgres.Querier) error {
		user, createErr := repo.CreateUser(ctx, postgres.CreateUserParams{
			ID:           shared.UUIDToPG(userID),
			Email:        req.User.Email,
			Username:     req.User.Username,
			PasswordHash: passwordHash,
		})
		if createErr != nil {
			return fmt.Errorf("create user: %w", createErr)
		}
		response, createErr = s.issueTokensForUser(ctx, &user, repo)
		return createErr
	})
	if err != nil {
		if !shared.IsExpectedError(err) {
			s.logger.LogAttrs(ctx, slog.LevelError, "auth repo err", shared.ErrorAttrs(err, slog.String("user_id", userID.String()))...)
		}
		return nil, err
	}
	return response, nil
}

func (s *Service) Login(ctx context.Context, req models.LoginUserRequest) (*models.UserResponse, error) {
	if req.User.Email == "" {
		return nil, shared.Validation("email", "can't be blank")
	}
	if req.User.Password == "" {
		return nil, shared.Validation("password", "can't be blank")
	}
	user, err := s.userRepo.GetUserByEmail(ctx, req.User.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, invalidCredentials()
		}
		s.logger.LogAttrs(ctx, slog.LevelError, "auth repo err", shared.ErrorAttrs(err)...)
		return nil, err
	}

	ok, err := s.passwordMgr.Verify(req.User.Password, user.PasswordHash)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "auth password err", shared.ErrorAttrs(err, shared.UUIDAttr("user_id", user.ID))...)
		return nil, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return nil, invalidCredentials()
	}

	return s.issueTokensForUser(ctx, &user, s.sessionRepo)
}

func (s *Service) RefreshToken(ctx context.Context, accessToken, refreshToken string) (*models.UserResponse, error) {
	claims, err := s.tokenMgr.ParseIgnoringExpiry(accessToken)
	if err != nil {
		return nil, fmt.Errorf("%w: parse access token", ErrInvalidToken)
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, fmt.Errorf("%w: not an access token", ErrInvalidToken)
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid user id in token", ErrInvalidToken)
	}
	jwtID, err := uuid.Parse(claims.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid jwt id in token", ErrInvalidToken)
	}
	refreshClaims, err := s.tokenMgr.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("%w: validate refresh token: %w", ErrInvalidToken, err)
	}
	if refreshClaims.UserID != claims.UserID {
		return nil, fmt.Errorf("%w: token subjects do not match", ErrInvalidToken)
	}

	session, err := s.sessionRepo.GetSessionByUserIDAndJWTIDAndRefreshToken(ctx, postgres.GetSessionByUserIDAndJWTIDAndRefreshTokenParams{
		UserID:       uuidToPGType(userID),
		JwtID:        uuidToPGType(jwtID),
		RefreshToken: hashRefreshToken(refreshToken),
	})
	if err != nil {
		if !shared.IsExpectedError(err) {
			s.logger.LogAttrs(ctx, slog.LevelError, "auth repo err", shared.ErrorAttrs(err, slog.String("user_id", userID.String()), slog.String("jwt_id", jwtID.String()))...)
		}
		return nil, fmt.Errorf("session not found: %w", err)
	}

	user, err := s.userRepo.GetUserByID(ctx, uuidToPGType(userID))
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "auth repo err", shared.ErrorAttrs(err, slog.String("user_id", userID.String()), slog.String("jwt_id", jwtID.String()), shared.UUIDAttr("session_id", session.ID))...)
		return nil, fmt.Errorf("get user: %w", err)
	}

	return s.rotateTokensForUser(ctx, &user, session.ID)
}

func (s *Service) ValidateAccessToken(token string) (*TokenClaims, error) {
	return s.tokenMgr.ValidateAccessToken(token)
}

func (s *Service) issueTokensForUser(ctx context.Context, user *postgres.User, sessions SessionRepository) (*models.UserResponse, error) {
	response, session, err := s.newTokenSession(ctx, user)
	if err != nil {
		return nil, err
	}
	if err := sessions.CreateSession(ctx, session); err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "auth repo err", shared.ErrorAttrs(err, shared.UUIDAttr("user_id", session.UserID), shared.UUIDAttr("session_id", session.ID), shared.UUIDAttr("jwt_id", session.JwtID))...)
		return nil, fmt.Errorf("create session: %w", err)
	}
	return response, nil
}

func (s *Service) rotateTokensForUser(ctx context.Context, user *postgres.User, oldSessionID pgtype.UUID) (*models.UserResponse, error) {
	response, session, err := s.newTokenSession(ctx, user)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "auth repo err", shared.ErrorAttrs(err, shared.UUIDAttr("user_id", session.UserID), shared.UUIDAttr("old_session_id", oldSessionID), shared.UUIDAttr("new_session_id", session.ID), shared.UUIDAttr("jwt_id", session.JwtID))...)
		return nil, err
	}
	rows, err := s.sessionRepo.RotateSession(ctx, postgres.RotateSessionParams{
		OldSessionID: oldSessionID,
		NewSessionID: session.ID,
		RefreshToken: session.RefreshToken,
		JwtID:        session.JwtID,
		UserID:       session.UserID,
		CreatedAt:    session.CreatedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("rotate session: %w", err)
	}
	if rows != 1 {
		return nil, fmt.Errorf("%w: session was already rotated", ErrInvalidToken)
	}
	return response, nil
}

func (s *Service) newTokenSession(ctx context.Context, user *postgres.User) (*models.UserResponse, postgres.CreateSessionParams, error) {
	userID, err := shared.PGToUUID(user.ID)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "auth uuid err", shared.ErrorAttrs(err, shared.UUIDAttr("user_id", user.ID))...)
		return nil, postgres.CreateSessionParams{}, fmt.Errorf("parse user id: %w", err)
	}

	pair, err := s.tokenMgr.GeneratePair(userID.String())
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "auth token err", shared.ErrorAttrs(err, slog.String("user_id", userID.String()))...)
		return nil, postgres.CreateSessionParams{}, fmt.Errorf("generate jwt pair: %w", err)
	}

	jwtUUID, err := uuid.Parse(pair.AccessClaims.ID)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "auth uuid err", shared.ErrorAttrs(err, slog.String("user_id", userID.String()))...)
		return nil, postgres.CreateSessionParams{}, fmt.Errorf("parse jwt id: %w", err)
	}

	session := postgres.CreateSessionParams{
		ID:           shared.NewUUID(),
		RefreshToken: hashRefreshToken(pair.RefreshToken),
		JwtID:        uuidToPGType(jwtUUID),
		UserID:       uuidToPGType(userID),
		CreatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}

	response := &models.UserResponse{
		User: models.User{
			Bio:          user.Bio.String,
			Email:        user.Email,
			Image:        user.Image.String,
			RefreshToken: pair.RefreshToken,
			Token:        pair.AccessToken,
			Username:     user.Username,
		},
	}
	return response, session, nil
}

func invalidCredentials() error {
	return &shared.APIError{Status: http.StatusUnauthorized, Field: "credentials", Message: "invalid"}
}

func hashRefreshToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
