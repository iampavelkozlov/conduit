package http

import (
	"errors"
	"testing"
	"time"

	api "conduit/internal/gen/http"
	"conduit/internal/models"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandlersPropagateErrorsAndMapSuccess(t *testing.T) {
	repoErr := errors.New("service error")
	article := models.Article{Slug: "slug", Title: "title", Description: "description", Body: "body", TagList: []string{"go"}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	profile := models.Profile{Username: "user"}
	user := models.User{Email: "user@example.com", Username: "user", Token: "token"}
	comment := models.Comment{ID: 7, Body: "body", Author: profile, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	articleResponse := &models.SingleArticleResponse{Article: article}
	profileResponse := &models.ProfileResponse{Profile: profile}
	userResponseModel := &models.UserResponse{User: user}

	tests := []struct {
		name   string
		expect func(*MockapplicationService, error)
		call   func(Server) (any, error)
	}{
		{name: "create article", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().CreateArticle(gomock.Any(), models.NewArticleRequest{Article: models.NewArticle{Title: "title", Description: "description", Body: "body"}}).Return(articleResponse, err)
		}, call: func(server Server) (any, error) {
			return server.CreateArticle(t.Context(), api.CreateArticleRequestObject{Body: new(api.CreateArticleJSONRequestBody{Article: api.NewArticle{Title: "title", Description: "description", Body: "body"}})})
		}},
		{name: "update article", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().UpdateArticle(gomock.Any(), "slug", gomock.Any()).Return(articleResponse, err)
		}, call: func(server Server) (any, error) {
			return server.UpdateArticle(t.Context(), api.UpdateArticleRequestObject{Slug: "slug", Body: new(api.UpdateArticleJSONRequestBody{})})
		}},
		{name: "delete article", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().DeleteArticle(gomock.Any(), "slug").Return(err)
		}, call: func(server Server) (any, error) {
			return server.DeleteArticle(t.Context(), api.DeleteArticleRequestObject{Slug: "slug"})
		}},
		{name: "get article", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().GetArticle(gomock.Any(), "slug").Return(articleResponse, err)
		}, call: func(server Server) (any, error) {
			return server.GetArticle(t.Context(), api.GetArticleRequestObject{Slug: "slug"})
		}},
		{name: "list articles", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().GetArticles(gomock.Any(), models.GetArticlesParams{}).Return(&models.MultipleArticlesResponse{Articles: []models.Article{article}, ArticlesCount: 1}, err)
		}, call: func(server Server) (any, error) {
			return server.GetArticles(t.Context(), api.GetArticlesRequestObject{})
		}},
		{name: "article feed", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().GetArticlesFeed(gomock.Any(), models.GetArticlesFeedParams{}).Return(&models.MultipleArticlesResponse{Articles: []models.Article{article}, ArticlesCount: 1}, err)
		}, call: func(server Server) (any, error) {
			return server.GetArticlesFeed(t.Context(), api.GetArticlesFeedRequestObject{})
		}},
		{name: "create favorite", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().CreateArticleFavorite(gomock.Any(), "slug").Return(articleResponse, err)
		}, call: func(server Server) (any, error) {
			return server.CreateArticleFavorite(t.Context(), api.CreateArticleFavoriteRequestObject{Slug: "slug"})
		}},
		{name: "delete favorite", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().DeleteArticleFavorite(gomock.Any(), "slug").Return(articleResponse, err)
		}, call: func(server Server) (any, error) {
			return server.DeleteArticleFavorite(t.Context(), api.DeleteArticleFavoriteRequestObject{Slug: "slug"})
		}},
		{name: "login", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().Login(gomock.Any(), models.LoginUserRequest{User: models.LoginUser{Email: "user@example.com", Password: "password"}}).Return(userResponseModel, err)
		}, call: func(server Server) (any, error) {
			return server.Login(t.Context(), api.LoginRequestObject{Body: new(api.LoginJSONRequestBody{User: api.LoginUser{Email: "user@example.com", Password: "password"}})})
		}},
		{name: "create comment", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().CreateArticleComment(gomock.Any(), "slug", models.NewCommentRequest{Comment: models.NewComment{Body: "body"}}).Return(&models.SingleCommentResponse{Comment: comment}, err)
		}, call: func(server Server) (any, error) {
			return server.CreateArticleComment(t.Context(), api.CreateArticleCommentRequestObject{Slug: "slug", Body: new(api.CreateArticleCommentJSONRequestBody{Comment: api.NewComment{Body: "body"}})})
		}},
		{name: "delete comment", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().DeleteArticleComment(gomock.Any(), "slug", 7).Return(err)
		}, call: func(server Server) (any, error) {
			return server.DeleteArticleComment(t.Context(), api.DeleteArticleCommentRequestObject{Slug: "slug", Id: 7})
		}},
		{name: "list comments", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().GetArticleComments(gomock.Any(), "slug").Return(&models.MultipleCommentsResponse{Comments: []models.Comment{comment}}, err)
		}, call: func(server Server) (any, error) {
			return server.GetArticleComments(t.Context(), api.GetArticleCommentsRequestObject{Slug: "slug"})
		}},
		{name: "create user", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().CreateUser(gomock.Any(), models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Password: "password", Username: "user"}}).Return(userResponseModel, err)
		}, call: func(server Server) (any, error) {
			return server.CreateUser(t.Context(), api.CreateUserRequestObject{Body: new(api.CreateUserJSONRequestBody{User: api.NewUser{Email: "user@example.com", Password: "password", Username: "user"}})})
		}},
		{name: "follow user", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().FollowUserByUsername(gomock.Any(), "user").Return(profileResponse, err)
		}, call: func(server Server) (any, error) {
			return server.FollowUserByUsername(t.Context(), api.FollowUserByUsernameRequestObject{Username: "user"})
		}},
		{name: "unfollow user", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().UnfollowUserByUsername(gomock.Any(), "user").Return(profileResponse, err)
		}, call: func(server Server) (any, error) {
			return server.UnfollowUserByUsername(t.Context(), api.UnfollowUserByUsernameRequestObject{Username: "user"})
		}},
		{name: "get profile", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().GetProfileByUsername(gomock.Any(), "user").Return(profileResponse, err)
		}, call: func(server Server) (any, error) {
			return server.GetProfileByUsername(t.Context(), api.GetProfileByUsernameRequestObject{Username: "user"})
		}},
		{name: "get current user", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().GetCurrentUser(gomock.Any()).Return(userResponseModel, err)
		}, call: func(server Server) (any, error) {
			return server.GetCurrentUser(t.Context(), api.GetCurrentUserRequestObject{})
		}},
		{name: "update current user", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().UpdateCurrentUser(gomock.Any(), gomock.Any()).Return(userResponseModel, err)
		}, call: func(server Server) (any, error) {
			return server.UpdateCurrentUser(t.Context(), api.UpdateCurrentUserRequestObject{Body: new(api.UpdateCurrentUserJSONRequestBody{})})
		}},
		{name: "get tags", expect: func(mock *MockapplicationService, err error) {
			mock.EXPECT().GetTags(gomock.Any()).Return(&models.TagsResponse{Tags: []string{"go"}}, err)
		}, call: func(server Server) (any, error) {
			return server.GetTags(t.Context(), api.GetTagsRequestObject{})
		}},
	}

	for _, tt := range tests {
		for _, fail := range []bool{false, true} {
			name := "success"
			var wantErr error
			if fail {
				name = "service error"
				wantErr = repoErr
			}
			t.Run(tt.name+"/"+name, func(t *testing.T) {
				mock := NewMockapplicationService(gomock.NewController(t))
				tt.expect(mock, wantErr)
				response, err := tt.call(Server{svc: mock})
				if wantErr != nil {
					require.ErrorIs(t, err, wantErr)
					require.Nil(t, response)
					return
				}
				require.NoError(t, err)
				require.NotNil(t, response)
			})
		}
	}
}
