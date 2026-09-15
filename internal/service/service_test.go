package service

import (
	"errors"
	"testing"

	"conduit/internal/models"
	"conduit/internal/service/auth"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type facadeMocks struct {
	article *MockarticleService
	auth    *MockauthService
	comment *MockcommentService
	tag     *MocktagService
	user    *MockuserService
}

func newMockFacade(t *testing.T) (*Service, facadeMocks) {
	ctrl := gomock.NewController(t)
	mocks := facadeMocks{
		article: NewMockarticleService(ctrl), auth: NewMockauthService(ctrl), comment: NewMockcommentService(ctrl),
		tag: NewMocktagService(ctrl), user: NewMockuserService(ctrl),
	}
	return &Service{article: mocks.article, auth: mocks.auth, comment: mocks.comment, tag: mocks.tag, user: mocks.user}, mocks
}

func TestServiceDelegatesSuccessAndErrors(t *testing.T) {
	dependencyErr := errors.New("dependency error")
	article := &models.SingleArticleResponse{}
	articles := &models.MultipleArticlesResponse{}
	user := &models.UserResponse{}
	profile := &models.ProfileResponse{}
	comment := &models.SingleCommentResponse{}
	comments := &models.MultipleCommentsResponse{}
	tags := &models.TagsResponse{}
	claims := &auth.TokenClaims{UserID: "user"}
	tests := []struct {
		name   string
		expect func(facadeMocks, error)
		call   func(*Service) error
	}{
		{name: "create article", expect: func(m facadeMocks, err error) {
			m.article.EXPECT().CreateArticle(gomock.Any(), models.NewArticleRequest{}).Return(article, err)
		}, call: func(s *Service) error { _, err := s.CreateArticle(t.Context(), models.NewArticleRequest{}); return err }},
		{name: "update article", expect: func(m facadeMocks, err error) {
			m.article.EXPECT().UpdateArticle(gomock.Any(), "slug", models.UpdateArticleRequest{}).Return(article, err)
		}, call: func(s *Service) error {
			_, err := s.UpdateArticle(t.Context(), "slug", models.UpdateArticleRequest{})
			return err
		}},
		{name: "delete article", expect: func(m facadeMocks, err error) { m.article.EXPECT().DeleteArticle(gomock.Any(), "slug").Return(err) }, call: func(s *Service) error { return s.DeleteArticle(t.Context(), "slug") }},
		{name: "get article", expect: func(m facadeMocks, err error) {
			m.article.EXPECT().GetArticle(gomock.Any(), "slug").Return(article, err)
		}, call: func(s *Service) error { _, err := s.GetArticle(t.Context(), "slug"); return err }},
		{name: "get articles", expect: func(m facadeMocks, err error) {
			m.article.EXPECT().GetArticles(gomock.Any(), models.GetArticlesParams{}).Return(articles, err)
		}, call: func(s *Service) error { _, err := s.GetArticles(t.Context(), models.GetArticlesParams{}); return err }},
		{name: "get feed", expect: func(m facadeMocks, err error) {
			m.article.EXPECT().GetArticlesFeed(gomock.Any(), models.GetArticlesFeedParams{}).Return(articles, err)
		}, call: func(s *Service) error {
			_, err := s.GetArticlesFeed(t.Context(), models.GetArticlesFeedParams{})
			return err
		}},
		{name: "create favorite", expect: func(m facadeMocks, err error) {
			m.article.EXPECT().CreateArticleFavorite(gomock.Any(), "slug").Return(article, err)
		}, call: func(s *Service) error { _, err := s.CreateArticleFavorite(t.Context(), "slug"); return err }},
		{name: "delete favorite", expect: func(m facadeMocks, err error) {
			m.article.EXPECT().DeleteArticleFavorite(gomock.Any(), "slug").Return(article, err)
		}, call: func(s *Service) error { _, err := s.DeleteArticleFavorite(t.Context(), "slug"); return err }},
		{name: "login", expect: func(m facadeMocks, err error) {
			m.auth.EXPECT().Login(gomock.Any(), models.LoginUserRequest{}).Return(user, err)
		}, call: func(s *Service) error { _, err := s.Login(t.Context(), models.LoginUserRequest{}); return err }},
		{name: "follow", expect: func(m facadeMocks, err error) {
			m.user.EXPECT().FollowUserByUsername(gomock.Any(), "user").Return(profile, err)
		}, call: func(s *Service) error { _, err := s.FollowUserByUsername(t.Context(), "user"); return err }},
		{name: "unfollow", expect: func(m facadeMocks, err error) {
			m.user.EXPECT().UnfollowUserByUsername(gomock.Any(), "user").Return(profile, err)
		}, call: func(s *Service) error { _, err := s.UnfollowUserByUsername(t.Context(), "user"); return err }},
		{name: "get profile", expect: func(m facadeMocks, err error) {
			m.user.EXPECT().GetProfileByUsername(gomock.Any(), "user").Return(profile, err)
		}, call: func(s *Service) error { _, err := s.GetProfileByUsername(t.Context(), "user"); return err }},
		{name: "refresh token", expect: func(m facadeMocks, err error) {
			m.auth.EXPECT().RefreshToken(gomock.Any(), "access", "refresh").Return(user, err)
		}, call: func(s *Service) error { _, err := s.RefreshToken(t.Context(), "access", "refresh"); return err }},
		{name: "create comment", expect: func(m facadeMocks, err error) {
			m.comment.EXPECT().CreateArticleComment(gomock.Any(), "slug", models.NewCommentRequest{}).Return(comment, err)
		}, call: func(s *Service) error {
			_, err := s.CreateArticleComment(t.Context(), "slug", models.NewCommentRequest{})
			return err
		}},
		{name: "delete comment", expect: func(m facadeMocks, err error) {
			m.comment.EXPECT().DeleteArticleComment(gomock.Any(), "slug", 7).Return(err)
		}, call: func(s *Service) error { return s.DeleteArticleComment(t.Context(), "slug", 7) }},
		{name: "get comments", expect: func(m facadeMocks, err error) {
			m.comment.EXPECT().GetArticleComments(gomock.Any(), "slug").Return(comments, err)
		}, call: func(s *Service) error { _, err := s.GetArticleComments(t.Context(), "slug"); return err }},
		{name: "create user", expect: func(m facadeMocks, err error) {
			m.auth.EXPECT().CreateUser(gomock.Any(), models.NewUserRequest{}).Return(user, err)
		}, call: func(s *Service) error { _, err := s.CreateUser(t.Context(), models.NewUserRequest{}); return err }},
		{name: "get current user", expect: func(m facadeMocks, err error) { m.user.EXPECT().GetCurrentUser(gomock.Any()).Return(user, err) }, call: func(s *Service) error { _, err := s.GetCurrentUser(t.Context()); return err }},
		{name: "update user", expect: func(m facadeMocks, err error) {
			m.user.EXPECT().UpdateCurrentUser(gomock.Any(), new(models.UpdateUserRequest)).Return(user, err)
		}, call: func(s *Service) error {
			_, err := s.UpdateCurrentUser(t.Context(), new(models.UpdateUserRequest))
			return err
		}},
		{name: "get tags", expect: func(m facadeMocks, err error) { m.tag.EXPECT().GetTags(gomock.Any()).Return(tags, err) }, call: func(s *Service) error { _, err := s.GetTags(t.Context()); return err }},
		{name: "validate access token", expect: func(m facadeMocks, err error) { m.auth.EXPECT().ValidateAccessToken("token").Return(claims, err) }, call: func(s *Service) error { _, err := s.ValidateAccessToken("token"); return err }},
	}

	for _, tt := range tests {
		for _, wantErr := range []error{nil, dependencyErr} {
			caseName := "success"
			if wantErr != nil {
				caseName = "dependency error"
			}
			t.Run(tt.name+"/"+caseName, func(t *testing.T) {
				service, mocks := newMockFacade(t)
				tt.expect(mocks, wantErr)
				err := tt.call(service)
				if wantErr != nil {
					require.ErrorIs(t, err, wantErr)
					return
				}
				require.NoError(t, err)
			})
		}
	}
}

func TestNew(t *testing.T) {
	require.NotNil(t, New(nil, nil, nil, nil, nil))
}
