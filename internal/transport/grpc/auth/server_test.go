package authgrpc

import (
	"testing"
	"time"

	authv1 "conduit/internal/gen/grpc/conduit/auth/v1"
	"conduit/internal/models"
	authservice "conduit/internal/service/auth"
	serviceshared "conduit/internal/service/shared"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRegister(t *testing.T) {
	server := grpc.NewServer()
	Register(server, NewMockservice(gomock.NewController(t)))
	_, ok := server.GetServiceInfo()[authv1.AuthService_ServiceDesc.ServiceName]
	require.True(t, ok)
}

func TestServerRegister(t *testing.T) {
	service := NewMockservice(gomock.NewController(t))
	userID := uuid.New()
	response := &models.UserResponse{User: models.User{Email: "user@example.com", Token: "access", RefreshToken: "refresh"}}
	expires := time.Now().Add(time.Hour)
	service.EXPECT().CreateUser(gomock.Any(), models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Password: "secret", Username: "user"}}).Return(response, nil)
	service.EXPECT().ValidateAccessToken("access").Return(&authservice.TokenClaims{UserID: userID.String(), RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expires)}}, nil)
	service.EXPECT().ValidateRefreshToken("refresh").Return(&authservice.TokenClaims{UserID: userID.String(), RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expires.Add(time.Hour))}}, nil)

	result, err := NewServer(service).Register(t.Context(), &authv1.RegisterRequest{Email: "user@example.com", Password: "secret", Username: "user"})
	require.NoError(t, err)
	require.Equal(t, userID.String(), result.GetAccount().GetUserId())
	require.Equal(t, "access", result.GetTokens().GetAccessToken())
	require.NotNil(t, result.GetTokens().GetAccessExpiresAt())
}

func TestServerLoginAndRefresh(t *testing.T) {
	userID := uuid.New()
	response := &models.UserResponse{User: models.User{Email: "user@example.com", Token: "access", RefreshToken: "refresh"}}
	claims := &authservice.TokenClaims{UserID: userID.String()}

	loginService := NewMockservice(gomock.NewController(t))
	loginService.EXPECT().Login(gomock.Any(), models.LoginUserRequest{User: models.LoginUser{Email: "user@example.com", Password: "secret"}}).Return(response, nil)
	loginService.EXPECT().ValidateAccessToken("access").Return(claims, nil)
	loginService.EXPECT().ValidateRefreshToken("refresh").Return(claims, nil)
	login, err := NewServer(loginService).Login(t.Context(), &authv1.LoginRequest{Email: "user@example.com", Password: "secret"})
	require.NoError(t, err)
	require.Equal(t, userID.String(), login.GetAccount().GetUserId())

	refreshService := NewMockservice(gomock.NewController(t))
	refreshService.EXPECT().RefreshToken(gomock.Any(), "old", "old-refresh").Return(response, nil)
	refreshService.EXPECT().ValidateAccessToken("access").Return(claims, nil)
	refreshService.EXPECT().ValidateRefreshToken("refresh").Return(claims, nil)
	refreshed, err := NewServer(refreshService).Refresh(t.Context(), &authv1.RefreshRequest{AccessToken: "old", RefreshToken: "old-refresh"})
	require.NoError(t, err)
	require.Equal(t, "refresh", refreshed.GetTokens().GetRefreshToken())
}

func TestServerRejectsInvalidIssuedTokens(t *testing.T) {
	response := &models.UserResponse{User: models.User{Token: "access", RefreshToken: "refresh"}}
	service := NewMockservice(gomock.NewController(t))
	service.EXPECT().Login(gomock.Any(), gomock.Any()).Return(response, nil)
	service.EXPECT().ValidateAccessToken("access").Return(nil, authservice.ErrInvalidToken)
	result, err := NewServer(service).Login(t.Context(), &authv1.LoginRequest{})
	require.Nil(t, result)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	service = NewMockservice(gomock.NewController(t))
	service.EXPECT().Login(gomock.Any(), gomock.Any()).Return(response, nil)
	service.EXPECT().ValidateAccessToken("access").Return(&authservice.TokenClaims{}, nil)
	service.EXPECT().ValidateRefreshToken("refresh").Return(nil, authservice.ErrExpiredToken)
	result, err = NewServer(service).Login(t.Context(), &authv1.LoginRequest{})
	require.Nil(t, result)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	service = NewMockservice(gomock.NewController(t))
	service.EXPECT().RefreshToken(gomock.Any(), "", "").Return(response, nil)
	service.EXPECT().ValidateAccessToken("access").Return(nil, authservice.ErrInvalidToken)
	refreshed, err := NewServer(service).Refresh(t.Context(), &authv1.RefreshRequest{})
	require.Nil(t, refreshed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	service = NewMockservice(gomock.NewController(t))
	service.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(response, nil)
	service.EXPECT().ValidateAccessToken("access").Return(nil, authservice.ErrInvalidToken)
	registered, err := NewServer(service).Register(t.Context(), &authv1.RegisterRequest{})
	require.Nil(t, registered)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestServerAuthOperationsMapErrors(t *testing.T) {
	tests := map[string]func(*Server) error{
		"register validation": func(server *Server) error {
			_, err := server.Register(t.Context(), &authv1.RegisterRequest{})
			return err
		},
		"login unauthorized": func(server *Server) error {
			_, err := server.Login(t.Context(), &authv1.LoginRequest{})
			return err
		},
		"refresh invalid token": func(server *Server) error {
			_, err := server.Refresh(t.Context(), &authv1.RefreshRequest{})
			return err
		},
	}
	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			service := NewMockservice(gomock.NewController(t))
			switch name {
			case "register validation":
				service.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(nil, serviceshared.ErrValidation)
			case "login unauthorized":
				service.EXPECT().Login(gomock.Any(), gomock.Any()).Return(nil, serviceshared.ErrUnauthorized)
			case "refresh invalid token":
				service.EXPECT().RefreshToken(gomock.Any(), "", "").Return(nil, authservice.ErrInvalidToken)
			}
			err := call(NewServer(service))
			require.NotEqual(t, codes.OK, status.Code(err))
		})
	}
}

func TestServerValidateAndAccounts(t *testing.T) {
	service := NewMockservice(gomock.NewController(t))
	userID := uuid.New()
	service.EXPECT().ValidateAccessToken("access").Return(&authservice.TokenClaims{
		UserID: userID.String(), RegisteredClaims: jwt.RegisteredClaims{ID: "token", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))},
	}, nil)
	validated, err := NewServer(service).ValidateAccessToken(t.Context(), &authv1.ValidateAccessTokenRequest{AccessToken: "access"})
	require.NoError(t, err)
	require.Equal(t, userID.String(), validated.GetUserId())
	service.EXPECT().ValidateAccessToken("access-without-expiry").Return(&authservice.TokenClaims{UserID: userID.String()}, nil)
	validated, err = NewServer(service).ValidateAccessToken(t.Context(), &authv1.ValidateAccessTokenRequest{AccessToken: "access-without-expiry"})
	require.NoError(t, err)
	require.Nil(t, validated.ExpiresAt)

	service.EXPECT().GetAccount(gomock.Any(), userID).Return(&authservice.Account{UserID: userID, Email: "old@example.com"}, nil)
	account, err := NewServer(service).GetAccount(t.Context(), &authv1.GetAccountRequest{UserId: userID.String()})
	require.NoError(t, err)
	require.Equal(t, "old@example.com", account.GetAccount().GetEmail())

	email := "new@example.com"
	service.EXPECT().UpdateCredentials(gomock.Any(), userID, &email, nil).Return(&authservice.Account{UserID: userID, Email: email}, nil)
	updated, err := NewServer(service).UpdateCredentials(t.Context(), &authv1.UpdateCredentialsRequest{UserId: userID.String(), Email: &email})
	require.NoError(t, err)
	require.Equal(t, email, updated.GetAccount().GetEmail())
}

func TestServerAccountOperationsMapServiceErrors(t *testing.T) {
	wantErr := serviceshared.ErrNotFound
	userID := uuid.New()
	service := NewMockservice(gomock.NewController(t))
	service.EXPECT().ValidateAccessToken("bad").Return(nil, authservice.ErrInvalidToken)
	_, err := NewServer(service).ValidateAccessToken(t.Context(), &authv1.ValidateAccessTokenRequest{AccessToken: "bad"})
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	service.EXPECT().GetAccount(gomock.Any(), userID).Return(nil, wantErr)
	_, err = NewServer(service).GetAccount(t.Context(), &authv1.GetAccountRequest{UserId: userID.String()})
	require.Equal(t, codes.NotFound, status.Code(err))

	service.EXPECT().UpdateCredentials(gomock.Any(), userID, nil, nil).Return(nil, wantErr)
	_, err = NewServer(service).UpdateCredentials(t.Context(), &authv1.UpdateCredentialsRequest{UserId: userID.String()})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestServerRejectsInvalidAccountID(t *testing.T) {
	server := NewServer(NewMockservice(gomock.NewController(t)))
	_, err := server.GetAccount(t.Context(), &authv1.GetAccountRequest{UserId: "invalid"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.UpdateCredentials(t.Context(), &authv1.UpdateCredentialsRequest{UserId: "invalid"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestServerDeleteAccount(t *testing.T) {
	userID := uuid.New()
	service := NewMockservice(gomock.NewController(t))
	service.EXPECT().DeleteAccount(gomock.Any(), userID).Return(nil)
	response, err := NewServer(service).DeleteAccount(t.Context(), &authv1.DeleteAccountRequest{UserId: userID.String()})
	require.NoError(t, err)
	require.True(t, response.GetDeleted())

	service.EXPECT().DeleteAccount(gomock.Any(), userID).Return(serviceshared.ErrNotFound)
	response, err = NewServer(service).DeleteAccount(t.Context(), &authv1.DeleteAccountRequest{UserId: userID.String()})
	require.Nil(t, response)
	require.Equal(t, codes.NotFound, status.Code(err))

	response, err = NewServer(service).DeleteAccount(t.Context(), &authv1.DeleteAccountRequest{UserId: "invalid"})
	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
