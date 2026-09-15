package auth

import (
	"errors"
	"fmt"
	"time"

	"conduit/internal/service/shared"

	jwt "github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type TokenClaims struct {
	UserID    string    `json:"user_id"`
	TokenType TokenType `json:"type"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken   string
	RefreshToken  string
	AccessClaims  TokenClaims
	RefreshClaims TokenClaims
}

type TokenManager struct {
	secret          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewTokenManager(secret string, accessTokenTTL, refreshTokenTTL time.Duration) *TokenManager {
	return &TokenManager{
		secret:          []byte(secret),
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}
}

func (m *TokenManager) GeneratePair(userID string) (*TokenPair, error) {
	now := time.Now().UTC()
	accessClaims := TokenClaims{
		UserID:    userID,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        shared.NewUUIDValue().String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTokenTTL)),
		},
	}
	refreshClaims := TokenClaims{
		UserID:    userID,
		TokenType: TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        shared.NewUUIDValue().String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTokenTTL)),
		},
	}

	accessToken, err := m.sign(&accessClaims)
	if err != nil {
		return nil, err
	}
	refreshToken, err := m.sign(&refreshClaims)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		AccessClaims:  accessClaims,
		RefreshClaims: refreshClaims,
	}, nil
}

func (m *TokenManager) ValidateAccessToken(token string) (*TokenClaims, error) {
	claims, err := m.validate(token)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (m *TokenManager) ValidateRefreshToken(token string) (*TokenClaims, error) {
	claims, err := m.validate(token)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeRefresh {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (m *TokenManager) validate(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(_ *jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) || errors.Is(err, jwt.ErrTokenMalformed) || errors.Is(err, jwt.ErrTokenUnverifiable) {
			return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
		}
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || claims.UserID == "" || claims.ID == "" {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ParseIgnoringExpiry validates the token signature but accepts expired tokens.
func (m *TokenManager) ParseIgnoringExpiry(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(_ *jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithoutClaimsValidation())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if token == nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || claims.UserID == "" || claims.ID == "" || claims.ExpiresAt == nil {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (m *TokenManager) sign(claims *TokenClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signedToken, nil
}
