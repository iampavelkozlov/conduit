package http

import (
	"context"

	"conduit/internal/models"
)

//go:generate go run go.uber.org/mock/mockgen -source=interfaces.go -destination=mock_http.go -package=http
type applicationService interface {
	CreateArticle(context.Context, models.NewArticleRequest) (*models.SingleArticleResponse, error)
	UpdateArticle(context.Context, string, models.UpdateArticleRequest) (*models.SingleArticleResponse, error)
	DeleteArticle(context.Context, string) error
	GetArticle(context.Context, string) (*models.SingleArticleResponse, error)
	GetArticles(context.Context, models.GetArticlesParams) (*models.MultipleArticlesResponse, error)
	GetArticlesFeed(context.Context, models.GetArticlesFeedParams) (*models.MultipleArticlesResponse, error)
	CreateArticleFavorite(context.Context, string) (*models.SingleArticleResponse, error)
	DeleteArticleFavorite(context.Context, string) (*models.SingleArticleResponse, error)
	Login(context.Context, models.LoginUserRequest) (*models.UserResponse, error)
	RefreshToken(context.Context, string, string) (*models.UserResponse, error)
	FollowUserByUsername(context.Context, string) (*models.ProfileResponse, error)
	UnfollowUserByUsername(context.Context, string) (*models.ProfileResponse, error)
	GetProfileByUsername(context.Context, string) (*models.ProfileResponse, error)
	CreateArticleComment(context.Context, string, models.NewCommentRequest) (*models.SingleCommentResponse, error)
	DeleteArticleComment(context.Context, string, int) error
	GetArticleComments(context.Context, string) (*models.MultipleCommentsResponse, error)
	CreateUser(context.Context, models.NewUserRequest) (*models.UserResponse, error)
	GetCurrentUser(context.Context) (*models.UserResponse, error)
	UpdateCurrentUser(context.Context, *models.UpdateUserRequest) (*models.UserResponse, error)
	GetTags(context.Context) (*models.TagsResponse, error)
}
