package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"conduit/internal/models"
	authservice "conduit/internal/service/auth"
	"conduit/internal/service/shared"
	authgrpc "conduit/internal/transport/grpc/auth"
	profilegrpc "conduit/internal/transport/grpc/profile"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateUserOrchestratesAuthThenProfile(t *testing.T) {
	auth := NewMockauthClient(gomock.NewController(t))
	profiles := NewMockprofileClient(gomock.NewController(t))
	userID := uuid.New()
	auth.EXPECT().Register(gomock.Any(), "user@example.com", "secret", "user").Return(&authgrpc.Result{Account: authservice.Account{UserID: userID, Email: "user@example.com"}, AccessToken: "access", RefreshToken: "refresh"}, nil)
	profiles.EXPECT().CreateProfile(gomock.Any(), userID, "user").Return(models.Profile{ID: userID, Username: "user", Bio: "bio"}, nil)
	service := New(auth, profiles, nil, nil, nil, time.Second)
	response, err := service.CreateUser(t.Context(), models.NewUserRequest{User: models.NewUser{Email: "user@example.com", Password: "secret", Username: "user"}})
	require.NoError(t, err)
	require.Equal(t, "access", response.User.Token)
	require.Equal(t, "user", response.User.Username)
}

func TestLoginAndRefreshComposeProfile(t *testing.T) {
	userID := uuid.New()
	for _, operation := range []string{"login", "refresh"} {
		t.Run(operation, func(t *testing.T) {
			auth := NewMockauthClient(gomock.NewController(t))
			profiles := NewMockprofileClient(gomock.NewController(t))
			result := &authgrpc.Result{Account: authservice.Account{UserID: userID, Email: "user@example.com"}, AccessToken: "access", RefreshToken: "refresh"}
			if operation == "login" {
				auth.EXPECT().Login(gomock.Any(), "user@example.com", "secret").Return(result, nil)
			} else {
				auth.EXPECT().Refresh(gomock.Any(), "old", "old-refresh").Return(result, nil)
			}
			profiles.EXPECT().GetProfile(gomock.Any(), userID).Return(models.Profile{ID: userID, Username: "user"}, nil)
			service := New(auth, profiles, nil, nil, nil, time.Second)
			var response *models.UserResponse
			var err error
			if operation == "login" {
				response, err = service.Login(t.Context(), models.LoginUserRequest{User: models.LoginUser{Email: "user@example.com", Password: "secret"}})
			} else {
				response, err = service.RefreshToken(t.Context(), "old", "old-refresh")
			}
			require.NoError(t, err)
			require.Equal(t, "user", response.User.Username)
		})
	}
}

func TestUpdateCurrentUserSplitsOwnershipAndPreservesNull(t *testing.T) {
	auth := NewMockauthClient(gomock.NewController(t))
	profiles := NewMockprofileClient(gomock.NewController(t))
	userID := uuid.New()
	email, username := "new@example.com", "new-name"
	profiles.EXPECT().GetProfileSnapshot(gomock.Any(), userID).Return(profilegrpc.Snapshot{Profile: models.Profile{ID: userID, Username: "old"}}, nil)
	auth.EXPECT().UpdateCredentials(gomock.Any(), userID, &email, nil).Return(&authservice.Account{UserID: userID, Email: email}, nil)
	profiles.EXPECT().UpdateProfile(gomock.Any(), userID, profilegrpc.Update{Username: &username, UsernameSet: true, BioSet: true}).Return(models.Profile{ID: userID, Username: username}, nil)
	ctx := shared.WithAccessToken(shared.WithUserID(t.Context(), userID), "access")
	response, err := New(auth, profiles, nil, nil, nil, time.Second).UpdateCurrentUser(ctx, &models.UpdateUserRequest{User: models.UpdateUser{
		Email: &email, EmailSet: true, Username: &username, UsernameSet: true, Bio: nil, BioSet: true,
	}})
	require.NoError(t, err)
	require.Equal(t, email, response.User.Email)
	require.Equal(t, "access", response.User.Token)
}

func TestUpdateCurrentUserRejectsNullRequiredFields(t *testing.T) {
	ctx := shared.WithUserID(t.Context(), uuid.New())
	service := New(NewMockauthClient(gomock.NewController(t)), NewMockprofileClient(gomock.NewController(t)), nil, nil, nil, time.Second)
	_, err := service.UpdateCurrentUser(ctx, &models.UpdateUserRequest{User: models.UpdateUser{EmailSet: true}})
	require.ErrorIs(t, err, shared.ErrValidation)
}

func TestProfileFollowOperations(t *testing.T) {
	profiles := NewMockprofileClient(gomock.NewController(t))
	subscriptions := NewMocksubscriptionsClient(gomock.NewController(t))
	viewerID, targetID := uuid.New(), uuid.New()
	ctx := shared.WithUserID(t.Context(), viewerID)
	profile := models.Profile{ID: targetID, Username: "target"}
	profiles.EXPECT().GetProfileByUsername(gomock.Any(), "target").Return(profile, nil)
	subscriptions.EXPECT().Follow(gomock.Any(), viewerID, targetID).Return(nil)
	response, err := New(nil, profiles, subscriptions, nil, nil, time.Second).FollowUserByUsername(ctx, "target")
	require.NoError(t, err)
	require.True(t, response.Profile.Following)

	profiles.EXPECT().GetProfileByUsername(gomock.Any(), "target").Return(profile, nil)
	subscriptions.EXPECT().Unfollow(gomock.Any(), viewerID, targetID).Return(nil)
	response, err = New(nil, profiles, subscriptions, nil, nil, time.Second).UnfollowUserByUsername(ctx, "target")
	require.NoError(t, err)
	require.False(t, response.Profile.Following)

	profiles.EXPECT().GetProfileByUsername(gomock.Any(), "target").Return(profile, nil)
	subscriptions.EXPECT().IsFollowing(gomock.Any(), viewerID, targetID).Return(true, nil)
	response, err = New(nil, profiles, subscriptions, nil, nil, time.Second).GetProfileByUsername(ctx, "target")
	require.NoError(t, err)
	require.True(t, response.Profile.Following)
}

func TestValidateAccessTokenUsesDeadline(t *testing.T) {
	auth := NewMockauthClient(gomock.NewController(t))
	auth.EXPECT().ValidateAccessToken(gomock.Any(), "token").DoAndReturn(func(ctx context.Context, _ string) (*authservice.TokenClaims, error) {
		_, ok := ctx.Deadline()
		require.True(t, ok)
		return &authservice.TokenClaims{UserID: uuid.NewString()}, nil
	})
	claims, err := New(auth, nil, nil, nil, nil, time.Second).ValidateAccessToken("token")
	require.NoError(t, err)
	require.NotEmpty(t, claims.UserID)
}

func TestArticleAndCommentMethodsDelegate(t *testing.T) {
	articles := NewMockarticleClient(gomock.NewController(t))
	comments := NewMockcommentClient(gomock.NewController(t))
	service := New(nil, nil, nil, articles, comments, time.Second)
	article := &models.SingleArticleResponse{}
	articleList := &models.MultipleArticlesResponse{}
	tags := &models.TagsResponse{}
	comment := &models.SingleCommentResponse{}
	commentList := &models.MultipleCommentsResponse{}

	articles.EXPECT().CreateArticle(gomock.Any(), models.NewArticleRequest{}).Return(article, nil)
	gotArticle, err := service.CreateArticle(t.Context(), models.NewArticleRequest{})
	require.NoError(t, err)
	require.Same(t, article, gotArticle)
	articles.EXPECT().UpdateArticle(gomock.Any(), "slug", models.UpdateArticleRequest{}).Return(article, nil)
	gotArticle, err = service.UpdateArticle(t.Context(), "slug", models.UpdateArticleRequest{})
	require.NoError(t, err)
	require.Same(t, article, gotArticle)
	articles.EXPECT().DeleteArticle(gomock.Any(), "slug").Return(nil)
	require.NoError(t, service.DeleteArticle(t.Context(), "slug"))
	articles.EXPECT().GetArticle(gomock.Any(), "slug").Return(article, nil)
	gotArticle, err = service.GetArticle(t.Context(), "slug")
	require.NoError(t, err)
	require.Same(t, article, gotArticle)
	articles.EXPECT().GetArticles(gomock.Any(), models.GetArticlesParams{}).Return(articleList, nil)
	gotArticles, err := service.GetArticles(t.Context(), models.GetArticlesParams{})
	require.NoError(t, err)
	require.Same(t, articleList, gotArticles)
	articles.EXPECT().GetArticlesFeed(gomock.Any(), models.GetArticlesFeedParams{}).Return(articleList, nil)
	gotArticles, err = service.GetArticlesFeed(t.Context(), models.GetArticlesFeedParams{})
	require.NoError(t, err)
	require.Same(t, articleList, gotArticles)
	articles.EXPECT().CreateArticleFavorite(gomock.Any(), "slug").Return(article, nil)
	gotArticle, err = service.CreateArticleFavorite(t.Context(), "slug")
	require.NoError(t, err)
	require.Same(t, article, gotArticle)
	articles.EXPECT().DeleteArticleFavorite(gomock.Any(), "slug").Return(article, nil)
	gotArticle, err = service.DeleteArticleFavorite(t.Context(), "slug")
	require.NoError(t, err)
	require.Same(t, article, gotArticle)
	articles.EXPECT().GetTags(gomock.Any()).Return(tags, nil)
	gotTags, err := service.GetTags(t.Context())
	require.NoError(t, err)
	require.Same(t, tags, gotTags)

	comments.EXPECT().CreateArticleComment(gomock.Any(), "slug", models.NewCommentRequest{}).Return(comment, nil)
	gotComment, err := service.CreateArticleComment(t.Context(), "slug", models.NewCommentRequest{})
	require.NoError(t, err)
	require.Same(t, comment, gotComment)
	comments.EXPECT().DeleteArticleComment(gomock.Any(), "slug", 7).Return(nil)
	require.NoError(t, service.DeleteArticleComment(t.Context(), "slug", 7))
	comments.EXPECT().GetArticleComments(gomock.Any(), "slug").Return(commentList, nil)
	gotComments, err := service.GetArticleComments(t.Context(), "slug")
	require.NoError(t, err)
	require.Same(t, commentList, gotComments)
}

func TestGetAndProfileOnlyUpdateCurrentUser(t *testing.T) {
	auth := NewMockauthClient(gomock.NewController(t))
	profiles := NewMockprofileClient(gomock.NewController(t))
	userID := uuid.New()
	account := &authservice.Account{UserID: userID, Email: "user@example.com"}
	profile := models.Profile{ID: userID, Username: "user"}
	auth.EXPECT().GetAccount(gomock.Any(), userID).Return(account, nil)
	profiles.EXPECT().GetProfile(gomock.Any(), userID).Return(profile, nil)
	ctx := shared.WithAccessToken(shared.WithUserID(t.Context(), userID), "token")
	current, err := New(auth, profiles, nil, nil, nil, 0).GetCurrentUser(ctx)
	require.NoError(t, err)
	require.Equal(t, "token", current.User.Token)
	auth.EXPECT().GetAccount(gomock.Any(), userID).Return(account, nil)
	profiles.EXPECT().GetProfileSnapshot(gomock.Any(), userID).Return(profilegrpc.Snapshot{Profile: profile}, nil)
	updated, err := New(auth, profiles, nil, nil, nil, 0).UpdateCurrentUser(ctx, &models.UpdateUserRequest{})
	require.NoError(t, err)
	require.Equal(t, "user", updated.User.Username)
}

func TestOrchestrationStopsOnDependencyFailure(t *testing.T) {
	wantErr := errors.New("dependency failed")
	auth := NewMockauthClient(gomock.NewController(t))
	auth.EXPECT().Register(gomock.Any(), "", "", "").Return(nil, wantErr)
	response, err := New(auth, NewMockprofileClient(gomock.NewController(t)), nil, nil, nil, time.Second).CreateUser(t.Context(), models.NewUserRequest{})
	require.Nil(t, response)
	require.ErrorIs(t, err, wantErr)

	_, err = New(nil, nil, nil, nil, nil, time.Second).GetCurrentUser(t.Context())
	require.ErrorIs(t, err, shared.ErrUnauthorized)
	_, err = New(nil, nil, nil, nil, nil, time.Second).FollowUserByUsername(t.Context(), "target")
	require.ErrorIs(t, err, shared.ErrUnauthorized)
}

func TestDependencyFailuresArePropagated(t *testing.T) {
	wantErr := errors.New("dependency failed")
	userID, targetID := uuid.New(), uuid.New()
	ctx := shared.WithUserID(t.Context(), userID)

	auth := NewMockauthClient(gomock.NewController(t))
	profiles := NewMockprofileClient(gomock.NewController(t))
	result := &authgrpc.Result{Account: authservice.Account{UserID: userID}}
	auth.EXPECT().Register(gomock.Any(), "", "", "").Return(result, nil)
	profiles.EXPECT().CreateProfile(gomock.Any(), userID, "").Return(models.Profile{}, wantErr)
	auth.EXPECT().DeleteAccount(gomock.Any(), userID).Return(nil)
	_, err := New(auth, profiles, nil, nil, nil, time.Second).CreateUser(t.Context(), models.NewUserRequest{})
	require.ErrorIs(t, err, wantErr)

	auth = NewMockauthClient(gomock.NewController(t))
	auth.EXPECT().Login(gomock.Any(), "", "").Return(nil, wantErr)
	_, err = New(auth, nil, nil, nil, nil, time.Second).Login(t.Context(), models.LoginUserRequest{})
	require.ErrorIs(t, err, wantErr)
	auth = NewMockauthClient(gomock.NewController(t))
	auth.EXPECT().Refresh(gomock.Any(), "", "").Return(nil, wantErr)
	_, err = New(auth, nil, nil, nil, nil, time.Second).RefreshToken(t.Context(), "", "")
	require.ErrorIs(t, err, wantErr)

	auth = NewMockauthClient(gomock.NewController(t))
	auth.EXPECT().GetAccount(gomock.Any(), userID).Return(nil, wantErr)
	_, err = New(auth, nil, nil, nil, nil, time.Second).GetCurrentUser(ctx)
	require.ErrorIs(t, err, wantErr)

	auth = NewMockauthClient(gomock.NewController(t))
	profiles = NewMockprofileClient(gomock.NewController(t))
	auth.EXPECT().GetAccount(gomock.Any(), userID).Return(&authservice.Account{UserID: userID}, nil)
	profiles.EXPECT().GetProfile(gomock.Any(), userID).Return(models.Profile{}, wantErr)
	_, err = New(auth, profiles, nil, nil, nil, time.Second).GetCurrentUser(ctx)
	require.ErrorIs(t, err, wantErr)

	_, err = New(nil, nil, nil, nil, nil, time.Second).UpdateCurrentUser(t.Context(), &models.UpdateUserRequest{})
	require.ErrorIs(t, err, shared.ErrUnauthorized)

	profiles = NewMockprofileClient(gomock.NewController(t))
	profiles.EXPECT().GetProfileSnapshot(gomock.Any(), userID).Return(profilegrpc.Snapshot{}, wantErr)
	_, err = New(nil, profiles, nil, nil, nil, time.Second).UpdateCurrentUser(ctx, &models.UpdateUserRequest{})
	require.ErrorIs(t, err, wantErr)

	auth = NewMockauthClient(gomock.NewController(t))
	profiles = NewMockprofileClient(gomock.NewController(t))
	profiles.EXPECT().GetProfileSnapshot(gomock.Any(), userID).Return(profilegrpc.Snapshot{}, nil)
	auth.EXPECT().GetAccount(gomock.Any(), userID).Return(nil, wantErr)
	_, err = New(auth, profiles, nil, nil, nil, time.Second).UpdateCurrentUser(ctx, &models.UpdateUserRequest{})
	require.ErrorIs(t, err, wantErr)

	profiles = NewMockprofileClient(gomock.NewController(t))
	profiles.EXPECT().GetProfileByUsername(gomock.Any(), "target").Return(models.Profile{ID: targetID}, wantErr)
	_, err = New(nil, profiles, nil, nil, nil, time.Second).FollowUserByUsername(ctx, "target")
	require.ErrorIs(t, err, wantErr)

	profiles = NewMockprofileClient(gomock.NewController(t))
	profiles.EXPECT().GetProfileByUsername(gomock.Any(), "public").Return(models.Profile{ID: targetID}, nil)
	response, err := New(nil, profiles, nil, nil, nil, time.Second).GetProfileByUsername(t.Context(), "public")
	require.NoError(t, err)
	require.False(t, response.Profile.Following)

	profiles = NewMockprofileClient(gomock.NewController(t))
	profiles.EXPECT().GetProfileByUsername(gomock.Any(), "public").Return(models.Profile{}, wantErr)
	_, err = New(nil, profiles, nil, nil, nil, time.Second).GetProfileByUsername(t.Context(), "public")
	require.ErrorIs(t, err, wantErr)

	profiles = NewMockprofileClient(gomock.NewController(t))
	subscriptions := NewMocksubscriptionsClient(gomock.NewController(t))
	profiles.EXPECT().GetProfileByUsername(gomock.Any(), "target").Return(models.Profile{ID: targetID}, nil)
	subscriptions.EXPECT().IsFollowing(gomock.Any(), userID, targetID).Return(false, wantErr)
	_, err = New(nil, profiles, subscriptions, nil, nil, time.Second).GetProfileByUsername(ctx, "target")
	require.ErrorIs(t, err, wantErr)

	profiles = NewMockprofileClient(gomock.NewController(t))
	subscriptions = NewMocksubscriptionsClient(gomock.NewController(t))
	profiles.EXPECT().GetProfileByUsername(gomock.Any(), "target").Return(models.Profile{ID: targetID}, nil)
	subscriptions.EXPECT().Follow(gomock.Any(), userID, targetID).Return(wantErr)
	_, err = New(nil, profiles, subscriptions, nil, nil, time.Second).FollowUserByUsername(ctx, "target")
	require.ErrorIs(t, err, wantErr)

	auth = NewMockauthClient(gomock.NewController(t))
	profiles = NewMockprofileClient(gomock.NewController(t))
	auth.EXPECT().Login(gomock.Any(), "", "").Return(result, nil)
	profiles.EXPECT().GetProfile(gomock.Any(), userID).Return(models.Profile{}, wantErr)
	_, err = New(auth, profiles, nil, nil, nil, time.Second).Login(t.Context(), models.LoginUserRequest{})
	require.ErrorIs(t, err, wantErr)

	auth = NewMockauthClient(gomock.NewController(t))
	profiles = NewMockprofileClient(gomock.NewController(t))
	profiles.EXPECT().GetProfileSnapshot(gomock.Any(), userID).Return(profilegrpc.Snapshot{Profile: models.Profile{ID: userID, Username: "old"}}, nil)
	profiles.EXPECT().UpdateProfile(gomock.Any(), userID, gomock.Any()).Return(models.Profile{}, wantErr)
	_, err = New(auth, profiles, nil, nil, nil, time.Second).UpdateCurrentUser(ctx, &models.UpdateUserRequest{User: models.UpdateUser{BioSet: true}})
	require.ErrorIs(t, err, wantErr)
}

func TestCreateUserJoinsCompensationFailure(t *testing.T) {
	profileErr := errors.New("create profile failed")
	compensationErr := errors.New("delete account failed")
	userID := uuid.New()
	auth := NewMockauthClient(gomock.NewController(t))
	profiles := NewMockprofileClient(gomock.NewController(t))
	auth.EXPECT().Register(gomock.Any(), "", "", "").Return(&authgrpc.Result{Account: authservice.Account{UserID: userID}}, nil)
	profiles.EXPECT().CreateProfile(gomock.Any(), userID, "").Return(models.Profile{}, profileErr)
	auth.EXPECT().DeleteAccount(gomock.Any(), userID).Return(compensationErr)
	response, err := New(auth, profiles, nil, nil, nil, time.Second).CreateUser(t.Context(), models.NewUserRequest{})
	require.Nil(t, response)
	require.ErrorIs(t, err, profileErr)
	require.ErrorIs(t, err, compensationErr)
}

func TestUpdateCurrentUserCompensatesProfileOnAuthFailure(t *testing.T) {
	userID := uuid.New()
	email, username := "new@example.com", "new"
	oldImage := "old-image"
	authErr := errors.New("update credentials failed")
	previous := profilegrpc.Snapshot{Profile: models.Profile{ID: userID, Username: "old", Image: oldImage}, Bio: nil, Image: &oldImage}
	ctx := shared.WithUserID(t.Context(), userID)

	for _, tt := range []struct {
		name            string
		compensationErr error
	}{
		{name: "rollback succeeds"},
		{name: "rollback fails", compensationErr: errors.New("restore profile failed")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auth := NewMockauthClient(gomock.NewController(t))
			profiles := NewMockprofileClient(gomock.NewController(t))
			profiles.EXPECT().GetProfileSnapshot(gomock.Any(), userID).Return(previous, nil)
			profiles.EXPECT().UpdateProfile(gomock.Any(), userID, profilegrpc.Update{Username: &username, UsernameSet: true}).Return(models.Profile{ID: userID, Username: username}, nil)
			auth.EXPECT().UpdateCredentials(gomock.Any(), userID, &email, nil).Return(nil, authErr)
			oldUsername := "old"
			profiles.EXPECT().UpdateProfile(gomock.Any(), userID, profilegrpc.Update{
				Username: &oldUsername, UsernameSet: true,
				Bio: nil, BioSet: true, Image: &oldImage, ImageSet: true,
			}).Return(previous.Profile, tt.compensationErr)
			response, err := New(auth, profiles, nil, nil, nil, time.Second).UpdateCurrentUser(ctx, &models.UpdateUserRequest{User: models.UpdateUser{
				Email: &email, EmailSet: true, Username: &username, UsernameSet: true,
			}})
			require.Nil(t, response)
			require.ErrorIs(t, err, authErr)
			if tt.compensationErr != nil {
				require.ErrorIs(t, err, tt.compensationErr)
			}
		})
	}
}
