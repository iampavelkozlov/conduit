package authgrpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	authv1 "conduit/internal/gen/grpc/conduit/auth/v1"
	authservice "conduit/internal/service/auth"
	serviceshared "conduit/internal/service/shared"
	grpcshared "conduit/internal/transport/grpc/shared"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Result struct {
	Account          authservice.Account
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

type Client struct{ client authv1.AuthServiceClient }

func NewClient(client authv1.AuthServiceClient) *Client { return &Client{client: client} }

func (c *Client) Register(ctx context.Context, email, password, username string) (*Result, error) {
	response, err := c.client.Register(ctx, &authv1.RegisterRequest{Email: email, Password: password, Username: username})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	return result(response.GetAccount(), response.GetTokens())
}

func (c *Client) Login(ctx context.Context, email, password string) (*Result, error) {
	response, err := c.client.Login(ctx, &authv1.LoginRequest{Email: email, Password: password})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	return result(response.GetAccount(), response.GetTokens())
}

func (c *Client) Refresh(ctx context.Context, accessToken, refreshToken string) (*Result, error) {
	response, err := c.client.Refresh(ctx, &authv1.RefreshRequest{AccessToken: accessToken, RefreshToken: refreshToken})
	if err != nil {
		decoded := grpcshared.DecodeError(err)
		if errors.Is(decoded, serviceshared.ErrUnauthorized) {
			return nil, authservice.ErrInvalidToken
		}
		return nil, decoded
	}
	return result(response.GetAccount(), response.GetTokens())
}

func (c *Client) ValidateAccessToken(ctx context.Context, token string) (*authservice.TokenClaims, error) {
	response, err := c.client.ValidateAccessToken(ctx, &authv1.ValidateAccessTokenRequest{AccessToken: token})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	claims := &authservice.TokenClaims{UserID: response.GetUserId(), TokenType: authservice.TokenTypeAccess}
	claims.ID = response.GetTokenId()
	if response.GetExpiresAt() != nil {
		claims.ExpiresAt = jwt.NewNumericDate(response.GetExpiresAt().AsTime())
	}
	return claims, nil
}

func (c *Client) GetAccount(ctx context.Context, userID uuid.UUID) (*authservice.Account, error) {
	response, err := c.client.GetAccount(ctx, &authv1.GetAccountRequest{UserId: userID.String()})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	return parseAccount(response.GetAccount())
}

func (c *Client) UpdateCredentials(ctx context.Context, userID uuid.UUID, email, password *string) (*authservice.Account, error) {
	response, err := c.client.UpdateCredentials(ctx, &authv1.UpdateCredentialsRequest{UserId: userID.String(), Email: email, Password: password})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	return parseAccount(response.GetAccount())
}

func (c *Client) DeleteAccount(ctx context.Context, userID uuid.UUID) error {
	_, err := c.client.DeleteAccount(ctx, &authv1.DeleteAccountRequest{UserId: userID.String()})
	return grpcshared.DecodeError(err)
}

func result(account *authv1.Account, tokens *authv1.TokenPair) (*Result, error) {
	parsed, err := parseAccount(account)
	if err != nil {
		return nil, err
	}
	if tokens == nil {
		return nil, errors.New("auth response has no token pair")
	}
	value := &Result{Account: *parsed, AccessToken: tokens.GetAccessToken(), RefreshToken: tokens.GetRefreshToken()}
	if tokens.GetAccessExpiresAt() != nil {
		value.AccessExpiresAt = tokens.GetAccessExpiresAt().AsTime()
	}
	if tokens.GetRefreshExpiresAt() != nil {
		value.RefreshExpiresAt = tokens.GetRefreshExpiresAt().AsTime()
	}
	return value, nil
}

func parseAccount(account *authv1.Account) (*authservice.Account, error) {
	if account == nil {
		return nil, errors.New("auth response has no account")
	}
	userID, err := uuid.Parse(account.GetUserId())
	if err != nil {
		return nil, fmt.Errorf("parse auth account UUID: %w", err)
	}
	return &authservice.Account{UserID: userID, Email: account.GetEmail()}, nil
}
