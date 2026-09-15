package service

import (
	"context"

	"conduit/internal/models"
	articlepkg "conduit/internal/service/article"
	authpkg "conduit/internal/service/auth"
	commentpkg "conduit/internal/service/comment"
	tagpkg "conduit/internal/service/tag"
	userpkg "conduit/internal/service/user"
)

type Service struct {
	article articleService
	auth    authService
	comment commentService
	tag     tagService
	user    userService
}

func New(
	articleService *articlepkg.Service,
	authService *authpkg.Service,
	commentService *commentpkg.Service,
	tagService *tagpkg.Service,
	userService *userpkg.Service,
) *Service {
	return &Service{
		article: articleService,
		auth:    authService,
		comment: commentService,
		tag:     tagService,
		user:    userService,
	}
}

func (s *Service) CreateArticle(ctx context.Context, req models.NewArticleRequest) (*models.SingleArticleResponse, error) {
	return s.article.CreateArticle(ctx, req)
}

func (s *Service) UpdateArticle(ctx context.Context, slug string, req models.UpdateArticleRequest) (*models.SingleArticleResponse, error) {
	return s.article.UpdateArticle(ctx, slug, req)
}

func (s *Service) DeleteArticle(ctx context.Context, slug string) error {
	return s.article.DeleteArticle(ctx, slug)
}

func (s *Service) GetArticle(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	return s.article.GetArticle(ctx, slug)
}

func (s *Service) GetArticles(ctx context.Context, params models.GetArticlesParams) (*models.MultipleArticlesResponse, error) {
	return s.article.GetArticles(ctx, params)
}

func (s *Service) GetArticlesFeed(ctx context.Context, params models.GetArticlesFeedParams) (*models.MultipleArticlesResponse, error) {
	return s.article.GetArticlesFeed(ctx, params)
}

func (s *Service) CreateArticleFavorite(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	return s.article.CreateArticleFavorite(ctx, slug)
}

func (s *Service) DeleteArticleFavorite(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	return s.article.DeleteArticleFavorite(ctx, slug)
}

func (s *Service) Login(ctx context.Context, req models.LoginUserRequest) (*models.UserResponse, error) {
	return s.auth.Login(ctx, req)
}

func (s *Service) FollowUserByUsername(ctx context.Context, username string) (*models.ProfileResponse, error) {
	return s.user.FollowUserByUsername(ctx, username)
}

func (s *Service) UnfollowUserByUsername(ctx context.Context, username string) (*models.ProfileResponse, error) {
	return s.user.UnfollowUserByUsername(ctx, username)
}

func (s *Service) GetProfileByUsername(ctx context.Context, username string) (*models.ProfileResponse, error) {
	return s.user.GetProfileByUsername(ctx, username)
}

func (s *Service) RefreshToken(ctx context.Context, accessToken, refreshToken string) (*models.UserResponse, error) {
	return s.auth.RefreshToken(ctx, accessToken, refreshToken)
}

func (s *Service) CreateArticleComment(ctx context.Context, slug string, req models.NewCommentRequest) (*models.SingleCommentResponse, error) {
	return s.comment.CreateArticleComment(ctx, slug, req)
}

func (s *Service) DeleteArticleComment(ctx context.Context, slug string, id int) error {
	return s.comment.DeleteArticleComment(ctx, slug, id)
}

func (s *Service) GetArticleComments(ctx context.Context, slug string) (*models.MultipleCommentsResponse, error) {
	return s.comment.GetArticleComments(ctx, slug)
}

func (s *Service) CreateUser(ctx context.Context, req models.NewUserRequest) (*models.UserResponse, error) {
	return s.auth.CreateUser(ctx, req)
}

func (s *Service) GetCurrentUser(ctx context.Context) (*models.UserResponse, error) {
	return s.user.GetCurrentUser(ctx)
}

func (s *Service) UpdateCurrentUser(ctx context.Context, req *models.UpdateUserRequest) (*models.UserResponse, error) {
	return s.user.UpdateCurrentUser(ctx, req)
}

func (s *Service) GetTags(ctx context.Context) (*models.TagsResponse, error) {
	return s.tag.GetTags(ctx)
}

func (s *Service) ValidateAccessToken(token string) (*authpkg.TokenClaims, error) {
	return s.auth.ValidateAccessToken(token)
}
