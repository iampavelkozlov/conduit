package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"

	"conduit/internal/models"
	authservice "conduit/internal/service/auth"
	"conduit/internal/service/shared"
	authgrpc "conduit/internal/transport/grpc/auth"
	profilegrpc "conduit/internal/transport/grpc/profile"

	"github.com/google/uuid"
)

//go:generate go run go.uber.org/mock/mockgen -source=service.go -destination=mock_gateway_test.go -package=gateway

type authClient interface {
	Register(context.Context, string, string, string) (*authgrpc.Result, error)
	Login(context.Context, string, string) (*authgrpc.Result, error)
	Refresh(context.Context, string, string) (*authgrpc.Result, error)
	ValidateAccessToken(context.Context, string) (*authservice.TokenClaims, error)
	GetAccount(context.Context, uuid.UUID) (*authservice.Account, error)
	UpdateCredentials(context.Context, uuid.UUID, *string, *string) (*authservice.Account, error)
	DeleteAccount(context.Context, uuid.UUID) error
}

type profileClient interface {
	CreateProfile(context.Context, uuid.UUID, string) (models.Profile, error)
	GetProfile(context.Context, uuid.UUID) (models.Profile, error)
	GetProfileSnapshot(context.Context, uuid.UUID) (profilegrpc.Snapshot, error)
	GetProfileByUsername(context.Context, string) (models.Profile, error)
	UpdateProfile(context.Context, uuid.UUID, profilegrpc.Update) (models.Profile, error)
}

type subscriptionsClient interface {
	Follow(context.Context, uuid.UUID, uuid.UUID) error
	Unfollow(context.Context, uuid.UUID, uuid.UUID) error
	IsFollowing(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

type articleClient interface {
	CreateArticle(context.Context, models.NewArticleRequest) (*models.SingleArticleResponse, error)
	UpdateArticle(context.Context, string, models.UpdateArticleRequest) (*models.SingleArticleResponse, error)
	DeleteArticle(context.Context, string) error
	GetArticle(context.Context, string) (*models.SingleArticleResponse, error)
	GetArticles(context.Context, models.GetArticlesParams) (*models.MultipleArticlesResponse, error)
	GetArticlesFeed(context.Context, models.GetArticlesFeedParams) (*models.MultipleArticlesResponse, error)
	CreateArticleFavorite(context.Context, string) (*models.SingleArticleResponse, error)
	DeleteArticleFavorite(context.Context, string) (*models.SingleArticleResponse, error)
	GetTags(context.Context) (*models.TagsResponse, error)
}

type commentClient interface {
	CreateArticleComment(context.Context, string, models.NewCommentRequest) (*models.SingleCommentResponse, error)
	DeleteArticleComment(context.Context, string, int) error
	GetArticleComments(context.Context, string) (*models.MultipleCommentsResponse, error)
}

type Service struct {
	auth          authClient
	profiles      profileClient
	subscriptions subscriptionsClient
	articles      articleClient
	comments      commentClient
	timeout       time.Duration
}

func New(auth authClient, profiles profileClient, subscriptions subscriptionsClient, articles articleClient, comments commentClient, timeout time.Duration) *Service {
	return &Service{auth: auth, profiles: profiles, subscriptions: subscriptions, articles: articles, comments: comments, timeout: timeout}
}

func (s *Service) CreateUser(ctx context.Context, request models.NewUserRequest) (*models.UserResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	result, err := s.auth.Register(ctx, request.User.Email, request.User.Password, request.User.Username)
	if err != nil {
		return nil, err
	}
	profile, err := s.profiles.CreateProfile(ctx, result.Account.UserID, request.User.Username)
	if err != nil {
		compensationCtx, compensationCancel := s.deadline(context.WithoutCancel(ctx))
		defer compensationCancel()
		if compensationErr := s.auth.DeleteAccount(compensationCtx, result.Account.UserID); compensationErr != nil {
			return nil, errors.Join(err, fmt.Errorf("delete auth account compensation: %w", compensationErr))
		}
		return nil, err
	}
	return userResponse(result, profile), nil
}

func (s *Service) Login(ctx context.Context, request models.LoginUserRequest) (*models.UserResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	result, err := s.auth.Login(ctx, request.User.Email, request.User.Password)
	if err != nil {
		return nil, err
	}
	return s.userResult(ctx, result)
}

func (s *Service) RefreshToken(ctx context.Context, accessToken, refreshToken string) (*models.UserResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	result, err := s.auth.Refresh(ctx, accessToken, refreshToken)
	if err != nil {
		return nil, err
	}
	return s.userResult(ctx, result)
}

func (s *Service) ValidateAccessToken(token string) (*authservice.TokenClaims, error) {
	ctx, cancel := s.deadline(context.Background())
	defer cancel()
	return s.auth.ValidateAccessToken(ctx, token)
}

func (s *Service) GetCurrentUser(ctx context.Context) (*models.UserResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	account, err := s.auth.GetAccount(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile, err := s.profiles.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &models.UserResponse{User: composeUser(account, profile, shared.AccessTokenFromContext(ctx), "")}, nil
}

func (s *Service) UpdateCurrentUser(ctx context.Context, request *models.UpdateUserRequest) (*models.UserResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	update := request.User
	if (update.EmailSet && update.Email == nil) || (update.PasswordSet && update.Password == nil) || (update.UsernameSet && update.Username == nil) {
		return nil, shared.ErrValidation
	}
	previous, err := s.profiles.GetProfileSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	profileUpdate := profilegrpc.Update{Username: update.Username, UsernameSet: update.UsernameSet, Bio: update.Bio, BioSet: update.BioSet, Image: update.Image, ImageSet: update.ImageSet}
	profile := previous.Profile
	profileChanged := update.UsernameSet || update.BioSet || update.ImageSet
	if profileChanged {
		profile, err = s.profiles.UpdateProfile(ctx, userID, profileUpdate)
	}
	if err != nil {
		return nil, err
	}
	var account *authservice.Account
	if update.EmailSet || update.PasswordSet {
		account, err = s.auth.UpdateCredentials(ctx, userID, update.Email, update.Password)
	} else {
		account, err = s.auth.GetAccount(ctx, userID)
	}
	if err != nil {
		if !profileChanged {
			return nil, err
		}
		compensationCtx, compensationCancel := s.deadline(context.WithoutCancel(ctx))
		defer compensationCancel()
		username := previous.Profile.Username
		rollback := profilegrpc.Update{Username: &username, UsernameSet: true, Bio: previous.Bio, BioSet: true, Image: previous.Image, ImageSet: true}
		if _, compensationErr := s.profiles.UpdateProfile(compensationCtx, userID, rollback); compensationErr != nil {
			return nil, errors.Join(err, fmt.Errorf("restore profile compensation: %w", compensationErr))
		}
		return nil, err
	}
	return &models.UserResponse{User: composeUser(account, profile, shared.AccessTokenFromContext(ctx), "")}, nil
}

func (s *Service) GetProfileByUsername(ctx context.Context, username string) (*models.ProfileResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	profile, err := s.profiles.GetProfileByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if viewerID, viewerErr := shared.UserIDFromContext(ctx); viewerErr == nil {
		profile.Following, err = s.subscriptions.IsFollowing(ctx, viewerID, profile.ID)
		if err != nil {
			return nil, err
		}
	}
	return &models.ProfileResponse{Profile: profile}, nil
}

func (s *Service) FollowUserByUsername(ctx context.Context, username string) (*models.ProfileResponse, error) {
	return s.changeFollow(ctx, username, true)
}

func (s *Service) UnfollowUserByUsername(ctx context.Context, username string) (*models.ProfileResponse, error) {
	return s.changeFollow(ctx, username, false)
}

func (s *Service) changeFollow(ctx context.Context, username string, follow bool) (*models.ProfileResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	viewerID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := s.profiles.GetProfileByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if follow {
		err = s.subscriptions.Follow(ctx, viewerID, profile.ID)
	} else {
		err = s.subscriptions.Unfollow(ctx, viewerID, profile.ID)
	}
	if err != nil {
		return nil, err
	}
	profile.Following = follow
	return &models.ProfileResponse{Profile: profile}, nil
}

func (s *Service) CreateArticle(ctx context.Context, request models.NewArticleRequest) (*models.SingleArticleResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	return s.articles.CreateArticle(ctx, request)
}
func (s *Service) UpdateArticle(ctx context.Context, slug string, request models.UpdateArticleRequest) (*models.SingleArticleResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	return s.articles.UpdateArticle(ctx, slug, request)
}
func (s *Service) DeleteArticle(ctx context.Context, slug string) error {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	return s.articles.DeleteArticle(ctx, slug)
}
func (s *Service) GetArticle(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	return s.articles.GetArticle(ctx, slug)
}
func (s *Service) GetArticles(ctx context.Context, params models.GetArticlesParams) (*models.MultipleArticlesResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	return s.articles.GetArticles(ctx, params)
}
func (s *Service) GetArticlesFeed(ctx context.Context, params models.GetArticlesFeedParams) (*models.MultipleArticlesResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	return s.articles.GetArticlesFeed(ctx, params)
}
func (s *Service) CreateArticleFavorite(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	return s.articles.CreateArticleFavorite(ctx, slug)
}
func (s *Service) DeleteArticleFavorite(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	return s.articles.DeleteArticleFavorite(ctx, slug)
}
func (s *Service) GetTags(ctx context.Context) (*models.TagsResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	return s.articles.GetTags(ctx)
}
func (s *Service) CreateArticleComment(ctx context.Context, slug string, request models.NewCommentRequest) (*models.SingleCommentResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	return s.comments.CreateArticleComment(ctx, slug, request)
}
func (s *Service) DeleteArticleComment(ctx context.Context, slug string, id int) error {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	return s.comments.DeleteArticleComment(ctx, slug, id)
}
func (s *Service) GetArticleComments(ctx context.Context, slug string) (*models.MultipleCommentsResponse, error) {
	ctx, cancel := s.deadline(ctx)
	defer cancel()
	return s.comments.GetArticleComments(ctx, slug)
}

func (s *Service) userResult(ctx context.Context, result *authgrpc.Result) (*models.UserResponse, error) {
	profile, err := s.profiles.GetProfile(ctx, result.Account.UserID)
	if err != nil {
		return nil, err
	}
	return userResponse(result, profile), nil
}

func userResponse(result *authgrpc.Result, profile models.Profile) *models.UserResponse {
	return &models.UserResponse{User: composeUser(&result.Account, profile, result.AccessToken, result.RefreshToken)}
}

func composeUser(account *authservice.Account, profile models.Profile, accessToken, refreshToken string) models.User {
	return models.User{Email: account.Email, Username: profile.Username, Bio: profile.Bio, Image: profile.Image, Token: accessToken, RefreshToken: refreshToken}
}

func currentUserID(ctx context.Context) (uuid.UUID, error) {
	id, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return uuid.Nil, shared.ErrUnauthorized
	}
	return id, nil
}

func (s *Service) deadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, s.timeout)
}
