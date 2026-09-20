package authgrpc

import (
	"context"
	"errors"

	authv1 "conduit/internal/gen/grpc/conduit/auth/v1"
	"conduit/internal/models"
	authservice "conduit/internal/service/auth"
	grpcshared "conduit/internal/transport/grpc/shared"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

//go:generate go run go.uber.org/mock/mockgen -source=server.go -destination=mock_service_test.go -package=authgrpc
type service interface {
	CreateUser(context.Context, models.NewUserRequest) (*models.UserResponse, error)
	Login(context.Context, models.LoginUserRequest) (*models.UserResponse, error)
	RefreshToken(context.Context, string, string) (*models.UserResponse, error)
	ValidateAccessToken(string) (*authservice.TokenClaims, error)
	ValidateRefreshToken(string) (*authservice.TokenClaims, error)
	GetAccount(context.Context, uuid.UUID) (*authservice.Account, error)
	UpdateCredentials(context.Context, uuid.UUID, *string, *string) (*authservice.Account, error)
	DeleteAccount(context.Context, uuid.UUID) error
}

type Server struct {
	authv1.UnimplementedAuthServiceServer
	service service
}

func NewServer(service service) *Server { return &Server{service: service} }

func Register(registrar grpc.ServiceRegistrar, service service) {
	var handler authv1.AuthServiceServer = NewServer(service)
	authv1.RegisterAuthServiceServer(registrar, handler)
}

func (s *Server) Register(ctx context.Context, request *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	response, err := s.service.CreateUser(ctx, models.NewUserRequest{User: models.NewUser{
		Email: request.GetEmail(), Password: request.GetPassword(), Username: request.GetUsername(),
	}})
	if err != nil {
		return nil, encodeError(err)
	}
	account, tokens, err := s.authResult(response)
	if err != nil {
		return nil, err
	}
	return &authv1.RegisterResponse{Account: account, Tokens: tokens}, nil
}

func (s *Server) Login(ctx context.Context, request *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	response, err := s.service.Login(ctx, models.LoginUserRequest{User: models.LoginUser{
		Email: request.GetEmail(), Password: request.GetPassword(),
	}})
	if err != nil {
		return nil, encodeError(err)
	}
	account, tokens, err := s.authResult(response)
	if err != nil {
		return nil, err
	}
	return &authv1.LoginResponse{Account: account, Tokens: tokens}, nil
}

func (s *Server) Refresh(ctx context.Context, request *authv1.RefreshRequest) (*authv1.RefreshResponse, error) {
	response, err := s.service.RefreshToken(ctx, request.GetAccessToken(), request.GetRefreshToken())
	if err != nil {
		return nil, encodeError(err)
	}
	account, tokens, err := s.authResult(response)
	if err != nil {
		return nil, err
	}
	return &authv1.RefreshResponse{Account: account, Tokens: tokens}, nil
}

func (s *Server) ValidateAccessToken(_ context.Context, request *authv1.ValidateAccessTokenRequest) (*authv1.ValidateAccessTokenResponse, error) {
	claims, err := s.service.ValidateAccessToken(request.GetAccessToken())
	if err != nil {
		return nil, encodeError(err)
	}
	response := &authv1.ValidateAccessTokenResponse{UserId: claims.UserID, TokenId: claims.ID}
	if claims.ExpiresAt != nil {
		response.ExpiresAt = timestamppb.New(claims.ExpiresAt.Time)
	}
	return response, nil
}

func (s *Server) GetAccount(ctx context.Context, request *authv1.GetAccountRequest) (*authv1.GetAccountResponse, error) {
	userID, err := parseUUID(request.GetUserId())
	if err != nil {
		return nil, err
	}
	account, err := s.service.GetAccount(ctx, userID)
	if err != nil {
		return nil, encodeError(err)
	}
	return &authv1.GetAccountResponse{Account: accountMessage(account)}, nil
}

func (s *Server) UpdateCredentials(ctx context.Context, request *authv1.UpdateCredentialsRequest) (*authv1.UpdateCredentialsResponse, error) {
	userID, err := parseUUID(request.GetUserId())
	if err != nil {
		return nil, err
	}
	account, err := s.service.UpdateCredentials(ctx, userID, request.Email, request.Password)
	if err != nil {
		return nil, encodeError(err)
	}
	return &authv1.UpdateCredentialsResponse{Account: accountMessage(account)}, nil
}

func (s *Server) DeleteAccount(ctx context.Context, request *authv1.DeleteAccountRequest) (*authv1.DeleteAccountResponse, error) {
	userID, err := parseUUID(request.GetUserId())
	if err != nil {
		return nil, err
	}
	if err := s.service.DeleteAccount(ctx, userID); err != nil {
		return nil, encodeError(err)
	}
	return &authv1.DeleteAccountResponse{Deleted: true}, nil
}

func (s *Server) authResult(response *models.UserResponse) (*authv1.Account, *authv1.TokenPair, error) {
	accessClaims, err := s.service.ValidateAccessToken(response.User.Token)
	if err != nil {
		return nil, nil, encodeError(err)
	}
	refreshClaims, err := s.service.ValidateRefreshToken(response.User.RefreshToken)
	if err != nil {
		return nil, nil, encodeError(err)
	}
	pair := &authv1.TokenPair{AccessToken: response.User.Token, RefreshToken: response.User.RefreshToken}
	if accessClaims.ExpiresAt != nil {
		pair.AccessExpiresAt = timestamppb.New(accessClaims.ExpiresAt.Time)
	}
	if refreshClaims.ExpiresAt != nil {
		pair.RefreshExpiresAt = timestamppb.New(refreshClaims.ExpiresAt.Time)
	}
	return &authv1.Account{UserId: accessClaims.UserID, Email: response.User.Email}, pair, nil
}

func accountMessage(account *authservice.Account) *authv1.Account {
	return &authv1.Account{UserId: account.UserID.String(), Email: account.Email}
}

func parseUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	return id, nil
}

func encodeError(err error) error {
	if errors.Is(err, authservice.ErrInvalidToken) || errors.Is(err, authservice.ErrExpiredToken) {
		return status.Error(codes.Unauthenticated, "invalid token")
	}
	return grpcshared.EncodeError(err)
}
