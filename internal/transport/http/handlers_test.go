package http

import (
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	api "conduit/internal/gen/http"
	"conduit/internal/models"
	"conduit/internal/service/auth"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandlersPropagateErrorsAndMapSuccess(t *testing.T) {
	repoErr := errors.New("service error")
	article := models.Article{Slug: "slug", Title: "title", Description: "description", Body: "body", TagList: []string{"go"}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	profile := models.Profile{Username: "user"}
	user := models.User{Email: "user@example.com", Username: "user", Token: "token", RefreshToken: "refresh-token"}
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

func TestAuthenticationResponsesSetRefreshCookie(t *testing.T) {
	mock := NewMockapplicationService(gomock.NewController(t))
	response := &models.UserResponse{User: models.User{Email: "user@example.com", Username: "user", Token: "access", RefreshToken: "refresh"}}
	mock.EXPECT().Login(gomock.Any(), gomock.Any()).Return(response, nil)
	mock.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(response, nil)
	server := Server{svc: mock}

	login, err := server.Login(t.Context(), api.LoginRequestObject{Body: new(api.LoginJSONRequestBody)})
	require.NoError(t, err)
	loginRecorder := httptest.NewRecorder()
	require.NoError(t, login.VisitLoginResponse(loginRecorder))
	require.NotContains(t, loginRecorder.Body.String(), "refresh")
	require.Equal(t, "refresh", loginRecorder.Result().Cookies()[0].Value)

	created, err := server.CreateUser(t.Context(), api.CreateUserRequestObject{Body: new(api.CreateUserJSONRequestBody)})
	require.NoError(t, err)
	createRecorder := httptest.NewRecorder()
	require.NoError(t, created.VisitCreateUserResponse(createRecorder))
	require.NotContains(t, createRecorder.Body.String(), "refresh")
	require.Equal(t, "refresh", createRecorder.Result().Cookies()[0].Value)
}

func TestRefreshHandler(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		cookie     string
		serviceErr error
		wantStatus int
		wantClear  bool
	}{
		{name: "missing access token", cookie: "refresh", wantStatus: nethttp.StatusUnauthorized},
		{name: "invalid access header", header: "Basic token", cookie: "refresh", wantStatus: nethttp.StatusUnauthorized},
		{name: "missing refresh token", header: "Token access", wantStatus: nethttp.StatusUnauthorized},
		{name: "invalid token pair", header: "Bearer access", cookie: "refresh", serviceErr: auth.ErrInvalidToken, wantStatus: nethttp.StatusUnauthorized, wantClear: true},
		{name: "repository failure", header: "Token access", cookie: "refresh", serviceErr: errors.New("database unavailable"), wantStatus: nethttp.StatusInternalServerError},
		{name: "rotates token pair", header: "Token access", cookie: "refresh", wantStatus: nethttp.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockapplicationService(gomock.NewController(t))
			if tt.header != "" && tt.cookie != "" && (tt.header == "Token access" || tt.header == "Bearer access") {
				mock.EXPECT().RefreshToken(gomock.Any(), "access", "refresh").Return(
					&models.UserResponse{User: models.User{Email: "user@example.com", Username: "user", Token: "new-access", RefreshToken: "new-refresh"}},
					tt.serviceErr,
				)
			}
			request := httptest.NewRequestWithContext(t.Context(), nethttp.MethodPost, "/internal/auth/refresh", nil)
			request.Header.Set("Authorization", tt.header)
			if tt.cookie != "" {
				request.AddCookie(&nethttp.Cookie{Name: refreshTokenCookieName, Value: tt.cookie})
			}
			recorder := httptest.NewRecorder()
			NewRefreshHandler(mock).ServeHTTP(recorder, request)
			require.Equal(t, tt.wantStatus, recorder.Code)
			if tt.wantClear {
				require.Equal(t, -1, recorder.Result().Cookies()[0].MaxAge)
			}
			if tt.wantStatus == nethttp.StatusOK {
				require.Contains(t, recorder.Body.String(), `"token":"new-access"`)
				require.NotContains(t, recorder.Body.String(), "new-refresh")
				require.Equal(t, "new-refresh", recorder.Result().Cookies()[0].Value)
			}
		})
	}
}
