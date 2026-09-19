package authgrpc

import (
	"testing"
	"time"

	authv1 "conduit/internal/gen/grpc/conduit/auth/v1"
	authservice "conduit/internal/service/auth"
	serviceshared "conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestClientAuthOperations(t *testing.T) {
	remote := NewMockAuthServiceClient(gomock.NewController(t))
	client := NewClient(remote)
	userID := uuid.New()
	expires := time.Now().UTC().Add(time.Hour)
	account := &authv1.Account{UserId: userID.String(), Email: "user@example.com"}
	tokens := &authv1.TokenPair{AccessToken: "access", RefreshToken: "refresh", AccessExpiresAt: timestamppb.New(expires), RefreshExpiresAt: timestamppb.New(expires.Add(time.Hour))}

	remote.EXPECT().Register(gomock.Any(), &authv1.RegisterRequest{Email: "user@example.com", Password: "secret", Username: "user"}).Return(&authv1.RegisterResponse{Account: account, Tokens: tokens}, nil)
	registered, err := client.Register(t.Context(), "user@example.com", "secret", "user")
	require.NoError(t, err)
	require.Equal(t, userID, registered.Account.UserID)
	require.Equal(t, "access", registered.AccessToken)

	remote.EXPECT().Login(gomock.Any(), &authv1.LoginRequest{Email: "user@example.com", Password: "secret"}).Return(&authv1.LoginResponse{Account: account, Tokens: tokens}, nil)
	loggedIn, err := client.Login(t.Context(), "user@example.com", "secret")
	require.NoError(t, err)
	require.Equal(t, "refresh", loggedIn.RefreshToken)

	remote.EXPECT().Refresh(gomock.Any(), &authv1.RefreshRequest{AccessToken: "old", RefreshToken: "old-refresh"}).Return(&authv1.RefreshResponse{Account: account, Tokens: tokens}, nil)
	refreshed, err := client.Refresh(t.Context(), "old", "old-refresh")
	require.NoError(t, err)
	require.Equal(t, expires.Unix(), refreshed.AccessExpiresAt.Unix())
}

func TestClientAccountOperations(t *testing.T) {
	remote := NewMockAuthServiceClient(gomock.NewController(t))
	client := NewClient(remote)
	userID := uuid.New()
	accountMessage := &authv1.Account{UserId: userID.String(), Email: "user@example.com"}
	remote.EXPECT().GetAccount(gomock.Any(), &authv1.GetAccountRequest{UserId: userID.String()}).Return(&authv1.GetAccountResponse{Account: accountMessage}, nil)
	account, err := client.GetAccount(t.Context(), userID)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", account.Email)

	email := "new@example.com"
	remote.EXPECT().UpdateCredentials(gomock.Any(), &authv1.UpdateCredentialsRequest{UserId: userID.String(), Email: &email}).Return(&authv1.UpdateCredentialsResponse{Account: &authv1.Account{UserId: userID.String(), Email: email}}, nil)
	account, err = client.UpdateCredentials(t.Context(), userID, &email, nil)
	require.NoError(t, err)
	require.Equal(t, email, account.Email)
	remote.EXPECT().DeleteAccount(gomock.Any(), &authv1.DeleteAccountRequest{UserId: userID.String()}).Return(&authv1.DeleteAccountResponse{Deleted: true}, nil)
	require.NoError(t, client.DeleteAccount(t.Context(), userID))
}

func TestClientValidateAccessToken(t *testing.T) {
	remote := NewMockAuthServiceClient(gomock.NewController(t))
	client := NewClient(remote)
	expires := time.Now().Add(time.Hour)
	remote.EXPECT().ValidateAccessToken(gomock.Any(), &authv1.ValidateAccessTokenRequest{AccessToken: "token"}).Return(&authv1.ValidateAccessTokenResponse{UserId: uuid.NewString(), TokenId: "id", ExpiresAt: timestamppb.New(expires)}, nil)
	claims, err := client.ValidateAccessToken(t.Context(), "token")
	require.NoError(t, err)
	require.Equal(t, "id", claims.ID)
	require.NotNil(t, claims.ExpiresAt)
}

func TestClientMapsRemoteErrors(t *testing.T) {
	remote := NewMockAuthServiceClient(gomock.NewController(t))
	client := NewClient(remote)
	remote.EXPECT().Login(gomock.Any(), gomock.Any()).Return(nil, status.Error(codes.Unauthenticated, "invalid"))
	result, err := client.Login(t.Context(), "user@example.com", "bad")
	require.Nil(t, result)
	require.ErrorIs(t, err, serviceshared.ErrUnauthorized)

	remote.EXPECT().Register(gomock.Any(), gomock.Any()).Return(nil, status.Error(codes.Unavailable, "offline"))
	_, err = client.Register(t.Context(), "", "", "")
	require.Equal(t, codes.Unavailable, status.Code(err))
	remote.EXPECT().Refresh(gomock.Any(), gomock.Any()).Return(nil, status.Error(codes.Unavailable, "offline"))
	_, err = client.Refresh(t.Context(), "", "")
	require.Equal(t, codes.Unavailable, status.Code(err))
	remote.EXPECT().Refresh(gomock.Any(), gomock.Any()).Return(nil, status.Error(codes.Unauthenticated, "invalid"))
	_, err = client.Refresh(t.Context(), "", "")
	require.ErrorIs(t, err, authservice.ErrInvalidToken)
	remote.EXPECT().ValidateAccessToken(gomock.Any(), gomock.Any()).Return(nil, status.Error(codes.Unauthenticated, "invalid"))
	_, err = client.ValidateAccessToken(t.Context(), "bad")
	require.ErrorIs(t, err, serviceshared.ErrUnauthorized)
	userID := uuid.New()
	remote.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil, status.Error(codes.NotFound, "missing"))
	_, err = client.GetAccount(t.Context(), userID)
	require.ErrorIs(t, err, serviceshared.ErrNotFound)
	remote.EXPECT().UpdateCredentials(gomock.Any(), gomock.Any()).Return(nil, status.Error(codes.PermissionDenied, "denied"))
	_, err = client.UpdateCredentials(t.Context(), userID, nil, nil)
	require.ErrorIs(t, err, serviceshared.ErrForbidden)
	remote.EXPECT().DeleteAccount(gomock.Any(), gomock.Any()).Return(nil, status.Error(codes.Unavailable, "offline"))
	require.Equal(t, codes.Unavailable, status.Code(client.DeleteAccount(t.Context(), userID)))
}

func TestClientRejectsMalformedResponses(t *testing.T) {
	_, err := result(nil, nil)
	require.ErrorContains(t, err, "no account")
	_, err = result(&authv1.Account{UserId: uuid.NewString()}, nil)
	require.ErrorContains(t, err, "no token pair")
	_, err = parseAccount(&authv1.Account{UserId: "invalid"})
	require.ErrorContains(t, err, "UUID")
}
