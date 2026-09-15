package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTokenManagerValidatesTokenTypes(t *testing.T) {
	manager := NewTokenManager("test-secret-that-is-at-least-32-bytes", time.Minute, time.Hour)
	pair, err := manager.GeneratePair(uuid.NewString())
	require.NoError(t, err)
	accessID, err := uuid.Parse(pair.AccessClaims.ID)
	require.NoError(t, err)
	refreshID, err := uuid.Parse(pair.RefreshClaims.ID)
	require.NoError(t, err)
	require.Equal(t, byte(8), accessID[6]>>4)
	require.Equal(t, byte(8), refreshID[6]>>4)

	_, err = manager.ValidateAccessToken(pair.AccessToken)
	require.NoError(t, err)
	_, err = manager.ValidateRefreshToken(pair.RefreshToken)
	require.NoError(t, err)

	_, err = manager.ValidateAccessToken(pair.RefreshToken)
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = manager.ValidateRefreshToken(pair.AccessToken)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTokenManagerRejectsExpiredAndTamperedTokens(t *testing.T) {
	manager := NewTokenManager("test-secret-that-is-at-least-32-bytes", -time.Minute, time.Hour)
	pair, err := manager.GeneratePair(uuid.NewString())
	require.NoError(t, err)

	_, err = manager.ValidateAccessToken(pair.AccessToken)
	require.ErrorIs(t, err, ErrExpiredToken)
	_, err = manager.ParseIgnoringExpiry(pair.AccessToken)
	require.NoError(t, err)

	tampered := pair.AccessToken[:len(pair.AccessToken)-2] + "xx"
	_, err = manager.ParseIgnoringExpiry(tampered)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTokenManagerRejectsUnexpectedSigningMethod(t *testing.T) {
	claims := TokenClaims{
		UserID:    uuid.NewString(),
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	manager := NewTokenManager("test-secret-that-is-at-least-32-bytes", time.Minute, time.Hour)
	_, err = manager.ParseIgnoringExpiry(token)
	require.ErrorIs(t, err, ErrInvalidToken)
}
