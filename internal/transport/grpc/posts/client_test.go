package posts

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	postsv1 "conduit/internal/gen/grpc/conduit/posts/v1"
	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestClientListArticlesHydratesAuthorsInOneBatch(t *testing.T) {
	remote := NewMockPostsServiceClient(gomock.NewController(t))
	profiles := NewMockprofileReader(gomock.NewController(t))
	authorID, articleOne, articleTwo := uuid.New(), uuid.New(), uuid.New()
	now := timestamppb.New(time.Now())
	remote.EXPECT().ListArticles(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, request *postsv1.ListArticlesRequest, _ ...any) (*postsv1.ListArticlesResponse, error) {
		require.Equal(t, uint32(20), request.GetPage().GetLimit())
		return &postsv1.ListArticlesResponse{ArticlesCount: 2, Articles: []*postsv1.Article{
			{Id: articleOne.String(), AuthorId: authorID.String(), Slug: "one", CreatedAt: now, UpdatedAt: now},
			{Id: articleTwo.String(), AuthorId: authorID.String(), Slug: "two", CreatedAt: now, UpdatedAt: now},
		}}, nil
	})
	profiles.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{authorID: {ID: authorID, Username: "author"}}, nil)
	response, err := NewApplicationClient(remote, profiles, nil).GetArticles(t.Context(), models.GetArticlesParams{})
	require.NoError(t, err)
	require.Equal(t, 2, response.ArticlesCount)
	require.Equal(t, "author", response.Articles[1].Author.Username)
}

func TestClientFeedUsesOneSubscriptionsAndProfileBatch(t *testing.T) {
	remote := NewMockPostsServiceClient(gomock.NewController(t))
	profiles := NewMockprofileReader(gomock.NewController(t))
	subscriptions := NewMocksubscriptionReader(gomock.NewController(t))
	viewerID, authorID, articleID := uuid.New(), uuid.New(), uuid.New()
	ctx := shared.WithUserID(t.Context(), viewerID)
	subscriptions.EXPECT().FolloweeIDs(gomock.Any(), viewerID).Return([]uuid.UUID{authorID}, nil)
	remote.EXPECT().ListFeed(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, request *postsv1.ListFeedRequest, _ ...any) (*postsv1.ListFeedResponse, error) {
		require.Equal(t, []string{authorID.String()}, request.GetAuthorIds())
		return &postsv1.ListFeedResponse{ArticlesCount: 1, Articles: []*postsv1.Article{{Id: articleID.String(), AuthorId: authorID.String(), CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()}}}, nil
	})
	profiles.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{authorID: {ID: authorID}}, nil)
	subscriptions.EXPECT().FollowingIDs(gomock.Any(), viewerID, []uuid.UUID{authorID}).Return(map[uuid.UUID]struct{}{authorID: {}}, nil)
	response, err := NewApplicationClient(remote, profiles, subscriptions).GetArticlesFeed(ctx, models.GetArticlesFeedParams{})
	require.NoError(t, err)
	require.Len(t, response.Articles, 1)
	require.True(t, response.Articles[0].Author.Following)
}

func TestClientValidatesLocalBoundary(t *testing.T) {
	client := NewApplicationClient(NewMockPostsServiceClient(gomock.NewController(t)), NewMockprofileReader(gomock.NewController(t)), nil)
	_, err := client.CreateArticle(t.Context(), models.NewArticleRequest{})
	require.ErrorIs(t, err, shared.ErrUnauthorized)
	ctx := shared.WithUserID(t.Context(), uuid.New())
	_, err = client.UpdateArticle(ctx, "slug", models.UpdateArticleRequest{Article: models.UpdateArticle{TitleSet: true}})
	require.ErrorIs(t, err, shared.ErrValidation)
}

func TestClientListTags(t *testing.T) {
	remote := NewMockPostsServiceClient(gomock.NewController(t))
	remote.EXPECT().ListTags(gomock.Any(), &postsv1.ListTagsRequest{}).Return(&postsv1.ListTagsResponse{Tags: []string{"go", "grpc"}}, nil)
	response, err := NewClient(remote).GetTags(t.Context())
	require.NoError(t, err)
	require.Equal(t, []string{"go", "grpc"}, response.Tags)
}

func TestClientArticleCommands(t *testing.T) {
	remote := NewMockPostsServiceClient(gomock.NewController(t))
	profiles := NewMockprofileReader(gomock.NewController(t))
	subscriptions := NewMocksubscriptionReader(gomock.NewController(t))
	userID, articleID := uuid.New(), uuid.New()
	article := &postsv1.Article{Id: articleID.String(), AuthorId: userID.String(), Slug: "slug", CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()}
	profiles.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{userID}).Return(map[uuid.UUID]models.Profile{userID: {ID: userID}}, nil).AnyTimes()
	subscriptions.EXPECT().FollowingIDs(gomock.Any(), userID, []uuid.UUID{userID}).Return(map[uuid.UUID]struct{}{}, nil).AnyTimes()
	client := NewApplicationClient(remote, profiles, subscriptions)
	ctx := shared.WithUserID(t.Context(), userID)

	remote.EXPECT().CreateArticle(gomock.Any(), gomock.Any()).Return(&postsv1.CreateArticleResponse{Article: article}, nil)
	_, err := client.CreateArticle(ctx, models.NewArticleRequest{Article: models.NewArticle{Title: "title"}})
	require.NoError(t, err)
	remote.EXPECT().GetArticle(gomock.Any(), &postsv1.GetArticleRequest{Slug: "slug", ViewerId: userID.String()}).Return(&postsv1.GetArticleResponse{Article: article}, nil)
	_, err = client.GetArticle(ctx, "slug")
	require.NoError(t, err)
	remote.EXPECT().UpdateArticle(gomock.Any(), gomock.Any()).Return(&postsv1.UpdateArticleResponse{Article: article}, nil)
	_, err = client.UpdateArticle(ctx, "slug", models.UpdateArticleRequest{})
	require.NoError(t, err)
	title, description, body := "title", "description", "body"
	tags := []string{"go"}
	remote.EXPECT().UpdateArticle(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, request *postsv1.UpdateArticleRequest, _ ...any) (*postsv1.UpdateArticleResponse, error) {
		require.Equal(t, title, request.GetTitle())
		require.Equal(t, description, request.GetDescription())
		require.Equal(t, body, request.GetBody())
		require.Equal(t, tags, request.GetTagList().GetValues())
		return &postsv1.UpdateArticleResponse{Article: article}, nil
	})
	_, err = client.UpdateArticle(ctx, "slug", models.UpdateArticleRequest{Article: models.UpdateArticle{
		Title: &title, TitleSet: true, Description: &description, DescriptionSet: true,
		Body: &body, BodySet: true, TagList: &tags, TagListSet: true,
	}})
	require.NoError(t, err)
	remote.EXPECT().DeleteArticle(gomock.Any(), &postsv1.DeleteArticleRequest{Slug: "slug", RequesterId: userID.String()}).Return(&postsv1.DeleteArticleResponse{Deleted: true}, nil)
	require.NoError(t, client.DeleteArticle(ctx, "slug"))
	remote.EXPECT().FavoriteArticle(gomock.Any(), &postsv1.FavoriteArticleRequest{Slug: "slug", UserId: userID.String()}).Return(&postsv1.FavoriteArticleResponse{Article: article}, nil)
	_, err = client.CreateArticleFavorite(ctx, "slug")
	require.NoError(t, err)
	remote.EXPECT().UnfavoriteArticle(gomock.Any(), &postsv1.UnfavoriteArticleRequest{Slug: "slug", UserId: userID.String()}).Return(&postsv1.UnfavoriteArticleResponse{Article: article}, nil)
	_, err = client.DeleteArticleFavorite(ctx, "slug")
	require.NoError(t, err)
}

func TestClientSuccessfulRPCWithInvalidPayloads(t *testing.T) {
	t.Parallel()
	remote := NewMockPostsServiceClient(gomock.NewController(t))
	profiles := NewMockprofileReader(gomock.NewController(t))
	subscriptions := NewMocksubscriptionReader(gomock.NewController(t))
	userID := uuid.New()
	ctx := shared.WithUserID(t.Context(), userID)
	client := NewApplicationClient(remote, profiles, subscriptions)
	_, err := client.UpdateArticle(t.Context(), "slug", models.UpdateArticleRequest{})
	require.ErrorIs(t, err, shared.ErrUnauthorized)

	remote.EXPECT().GetArticle(gomock.Any(), gomock.Any()).Return(&postsv1.GetArticleResponse{}, nil)
	_, err = client.GetArticle(ctx, "slug")
	require.ErrorContains(t, err, "nil article")
	remote.EXPECT().ListArticles(gomock.Any(), gomock.Any()).Return(&postsv1.ListArticlesResponse{Articles: []*postsv1.Article{nil}}, nil)
	_, err = client.GetArticles(ctx, models.GetArticlesParams{})
	require.ErrorContains(t, err, "nil article")
	remote.EXPECT().ListArticles(gomock.Any(), gomock.Any()).Return(&postsv1.ListArticlesResponse{ArticlesCount: math.MaxUint64}, nil)
	_, err = client.GetArticles(ctx, models.GetArticlesParams{})
	require.ErrorContains(t, err, "overflows int")
	subscriptions.EXPECT().FolloweeIDs(gomock.Any(), userID).Return(nil, nil)
	remote.EXPECT().ListFeed(gomock.Any(), gomock.Any()).Return(&postsv1.ListFeedResponse{Articles: []*postsv1.Article{nil}}, nil)
	_, err = client.GetArticlesFeed(ctx, models.GetArticlesFeedParams{})
	require.ErrorContains(t, err, "nil article")
	subscriptions.EXPECT().FolloweeIDs(gomock.Any(), userID).Return(nil, nil)
	remote.EXPECT().ListFeed(gomock.Any(), gomock.Any()).Return(&postsv1.ListFeedResponse{ArticlesCount: math.MaxUint64}, nil)
	_, err = client.GetArticlesFeed(ctx, models.GetArticlesFeedParams{})
	require.ErrorContains(t, err, "overflows int")
	remote.EXPECT().FavoriteArticle(gomock.Any(), gomock.Any()).Return(&postsv1.FavoriteArticleResponse{}, nil)
	_, err = client.CreateArticleFavorite(ctx, "slug")
	require.ErrorContains(t, err, "nil article")
	remote.EXPECT().UnfavoriteArticle(gomock.Any(), gomock.Any()).Return(&postsv1.UnfavoriteArticleResponse{}, nil)
	_, err = client.DeleteArticleFavorite(ctx, "slug")
	require.ErrorContains(t, err, "nil article")

	authorID, articleID := uuid.New(), uuid.New()
	profiles.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{authorID: {ID: authorID}}, nil)
	_, err = NewApplicationClient(remote, profiles, nil).hydrate(t.Context(), []*postsv1.Article{{
		Id: articleID.String(), AuthorId: authorID.String(), FavoritesCount: math.MaxUint64,
	}})
	require.ErrorContains(t, err, "overflows int")
}

func TestClientResolveAndFilteredList(t *testing.T) {
	remote := NewMockPostsServiceClient(gomock.NewController(t))
	profiles := NewMockprofileReader(gomock.NewController(t))
	authorID, favoritedID, articleID := uuid.New(), uuid.New(), uuid.New()
	remote.EXPECT().ResolveArticleID(gomock.Any(), &postsv1.ResolveArticleIDRequest{Slug: "slug"}).Return(&postsv1.ResolveArticleIDResponse{ArticleId: articleID.String()}, nil)
	resolved, err := NewClient(remote).ResolveArticleID(t.Context(), "slug")
	require.NoError(t, err)
	require.Equal(t, articleID, resolved)
	profiles.EXPECT().IDByUsername(gomock.Any(), "author").Return(authorID, nil)
	profiles.EXPECT().IDByUsername(gomock.Any(), "fan").Return(favoritedID, nil)
	remote.EXPECT().ListArticles(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, request *postsv1.ListArticlesRequest, _ ...any) (*postsv1.ListArticlesResponse, error) {
		require.Equal(t, authorID.String(), request.GetAuthorId())
		require.Equal(t, favoritedID.String(), request.GetFavoritedByUserId())
		return &postsv1.ListArticlesResponse{}, nil
	})
	author, fan := "author", "fan"
	_, err = NewApplicationClient(remote, profiles, nil).GetArticles(t.Context(), models.GetArticlesParams{Author: &author, Favorited: &fan})
	require.NoError(t, err)
}

func TestClientRejectsMalformedArticleResponses(t *testing.T) {
	profiles := NewMockprofileReader(gomock.NewController(t))
	client := NewApplicationClient(NewMockPostsServiceClient(gomock.NewController(t)), profiles, nil)
	_, err := client.hydrate(t.Context(), []*postsv1.Article{nil})
	require.ErrorContains(t, err, "nil article")
	_, err = client.hydrate(t.Context(), []*postsv1.Article{{AuthorId: "bad"}})
	require.ErrorContains(t, err, "author ID")
	authorID := uuid.New()
	profiles.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{}, nil)
	_, err = client.hydrate(t.Context(), []*postsv1.Article{{AuthorId: authorID.String()}})
	require.ErrorContains(t, err, "was not returned")
	_, err = safeInt(math.MaxUint64)
	require.ErrorContains(t, err, "overflows")
	negative, tooLarge := -1, 101
	require.Equal(t, uint32(20), page(&negative, &negative).GetLimit())
	require.Equal(t, uint32(20), page(&tooLarge, nil).GetLimit())
}

func TestClientRemoteAndDependencyErrors(t *testing.T) {
	remoteErr := status.Error(codes.Internal, "remote")
	dependencyErr := errors.New("dependency")
	userID, authorID, articleID := uuid.New(), uuid.New(), uuid.New()
	unauthenticatedCtx := context.WithoutCancel(t.Context())
	ctx := shared.WithUserID(unauthenticatedCtx, userID)

	t.Run("resolve", func(t *testing.T) {
		remote := NewMockPostsServiceClient(gomock.NewController(t))
		remote.EXPECT().ResolveArticleID(gomock.Any(), gomock.Any()).Return(nil, remoteErr)
		_, err := NewClient(remote).ResolveArticleID(t.Context(), "slug")
		require.Error(t, err)
		remote.EXPECT().ResolveArticleID(gomock.Any(), gomock.Any()).Return(&postsv1.ResolveArticleIDResponse{ArticleId: "bad"}, nil)
		_, err = NewClient(remote).ResolveArticleID(t.Context(), "slug")
		require.ErrorContains(t, err, "article ID")
	})

	t.Run("commands", func(t *testing.T) {
		remote := NewMockPostsServiceClient(gomock.NewController(t))
		profiles := NewMockprofileReader(gomock.NewController(t))
		client := NewApplicationClient(remote, profiles, nil)
		remote.EXPECT().CreateArticle(gomock.Any(), gomock.Any()).Return(nil, remoteErr)
		tags := []string{"go"}
		_, err := client.CreateArticle(ctx, models.NewArticleRequest{Article: models.NewArticle{TagList: &tags}})
		require.Error(t, err)
		remote.EXPECT().GetArticle(gomock.Any(), gomock.Any()).Return(nil, remoteErr)
		_, err = client.GetArticle(ctx, "slug")
		require.Error(t, err)
		remote.EXPECT().UpdateArticle(gomock.Any(), gomock.Any()).Return(nil, remoteErr)
		_, err = client.UpdateArticle(ctx, "slug", models.UpdateArticleRequest{})
		require.Error(t, err)
		remote.EXPECT().DeleteArticle(gomock.Any(), gomock.Any()).Return(nil, remoteErr)
		require.Error(t, client.DeleteArticle(ctx, "slug"))
		remote.EXPECT().FavoriteArticle(gomock.Any(), gomock.Any()).Return(nil, remoteErr)
		_, err = client.CreateArticleFavorite(ctx, "slug")
		require.Error(t, err)
		remote.EXPECT().UnfavoriteArticle(gomock.Any(), gomock.Any()).Return(nil, remoteErr)
		_, err = client.DeleteArticleFavorite(ctx, "slug")
		require.Error(t, err)
		remote.EXPECT().ListTags(gomock.Any(), gomock.Any()).Return(nil, remoteErr)
		_, err = client.GetTags(ctx)
		require.Error(t, err)
	})

	t.Run("authorization and update validation", func(t *testing.T) {
		client := NewApplicationClient(NewMockPostsServiceClient(gomock.NewController(t)), NewMockprofileReader(gomock.NewController(t)), NewMocksubscriptionReader(gomock.NewController(t)))
		require.ErrorIs(t, client.DeleteArticle(unauthenticatedCtx, "slug"), shared.ErrUnauthorized)
		_, err := client.GetArticlesFeed(unauthenticatedCtx, models.GetArticlesFeedParams{})
		require.ErrorIs(t, err, shared.ErrUnauthorized)
		_, err = client.CreateArticleFavorite(unauthenticatedCtx, "slug")
		require.ErrorIs(t, err, shared.ErrUnauthorized)
		_, err = client.DeleteArticleFavorite(unauthenticatedCtx, "slug")
		require.ErrorIs(t, err, shared.ErrUnauthorized)
		_, err = client.UpdateArticle(ctx, "slug", models.UpdateArticleRequest{Article: models.UpdateArticle{DescriptionSet: true}})
		require.ErrorIs(t, err, shared.ErrValidation)
		_, err = client.UpdateArticle(ctx, "slug", models.UpdateArticleRequest{Article: models.UpdateArticle{BodySet: true}})
		require.ErrorIs(t, err, shared.ErrValidation)
		_, err = client.UpdateArticle(ctx, "slug", models.UpdateArticleRequest{Article: models.UpdateArticle{TagListSet: true}})
		require.ErrorIs(t, err, shared.ErrValidation)
	})

	t.Run("list filters", func(t *testing.T) {
		remote := NewMockPostsServiceClient(gomock.NewController(t))
		profiles := NewMockprofileReader(gomock.NewController(t))
		client := NewApplicationClient(remote, profiles, nil)
		author := "author"
		profiles.EXPECT().IDByUsername(gomock.Any(), author).Return(uuid.Nil, dependencyErr)
		_, err := client.GetArticles(t.Context(), models.GetArticlesParams{Author: &author})
		require.ErrorIs(t, err, dependencyErr)
		fan := "fan"
		profiles.EXPECT().IDByUsername(gomock.Any(), fan).Return(uuid.Nil, dependencyErr)
		_, err = client.GetArticles(t.Context(), models.GetArticlesParams{Favorited: &fan})
		require.ErrorIs(t, err, dependencyErr)
		remote.EXPECT().ListArticles(gomock.Any(), gomock.Any()).Return(nil, remoteErr)
		_, err = client.GetArticles(t.Context(), models.GetArticlesParams{})
		require.Error(t, err)
	})

	t.Run("feed dependencies", func(t *testing.T) {
		remote := NewMockPostsServiceClient(gomock.NewController(t))
		profiles := NewMockprofileReader(gomock.NewController(t))
		subscriptions := NewMocksubscriptionReader(gomock.NewController(t))
		client := NewApplicationClient(remote, profiles, subscriptions)
		subscriptions.EXPECT().FolloweeIDs(gomock.Any(), userID).Return(nil, dependencyErr)
		_, err := client.GetArticlesFeed(ctx, models.GetArticlesFeedParams{})
		require.ErrorIs(t, err, dependencyErr)
		subscriptions.EXPECT().FolloweeIDs(gomock.Any(), userID).Return([]uuid.UUID{authorID}, nil)
		remote.EXPECT().ListFeed(gomock.Any(), gomock.Any()).Return(nil, remoteErr)
		_, err = client.GetArticlesFeed(ctx, models.GetArticlesFeedParams{})
		require.Error(t, err)
	})

	t.Run("hydration", func(t *testing.T) {
		profiles := NewMockprofileReader(gomock.NewController(t))
		subscriptions := NewMocksubscriptionReader(gomock.NewController(t))
		client := NewApplicationClient(NewMockPostsServiceClient(gomock.NewController(t)), profiles, subscriptions)
		article := &postsv1.Article{Id: articleID.String(), AuthorId: authorID.String(), CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()}
		profiles.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(nil, dependencyErr)
		_, err := client.hydrate(unauthenticatedCtx, []*postsv1.Article{article})
		require.ErrorIs(t, err, dependencyErr)
		profiles.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{authorID: {ID: authorID}}, nil)
		subscriptions.EXPECT().FollowingIDs(gomock.Any(), userID, []uuid.UUID{authorID}).Return(nil, dependencyErr)
		_, err = client.hydrate(ctx, []*postsv1.Article{article})
		require.ErrorIs(t, err, dependencyErr)
		badID := &postsv1.Article{Id: "bad", AuthorId: authorID.String(), CreatedAt: article.GetCreatedAt(), UpdatedAt: article.GetUpdatedAt()}
		profiles.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{authorID: {ID: authorID}}, nil)
		_, err = NewApplicationClient(NewMockPostsServiceClient(gomock.NewController(t)), profiles, nil).hydrate(unauthenticatedCtx, []*postsv1.Article{badID})
		require.ErrorContains(t, err, "parse article ID")
		profiles = NewMockprofileReader(gomock.NewController(t))
		profiles.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{authorID: {ID: authorID}}, nil)
		hydrated, err := NewApplicationClient(NewMockPostsServiceClient(gomock.NewController(t)), profiles, nil).hydrate(unauthenticatedCtx, []*postsv1.Article{article})
		require.NoError(t, err)
		require.NotNil(t, hydrated[0].TagList)
		require.Empty(t, hydrated[0].TagList)
		require.Empty(t, mustHydrateEmpty(t, unauthenticatedCtx, client))
	})
}

func TestPageUsesValidValues(t *testing.T) {
	t.Parallel()
	limit, offset := 50, 7
	got := page(&limit, &offset)
	require.Equal(t, uint32(50), got.GetLimit())
	require.Equal(t, uint32(7), got.GetOffset())
}

func mustHydrateEmpty(t *testing.T, ctx context.Context, client *Client) []models.Article {
	t.Helper()
	values, err := client.hydrate(ctx, nil)
	require.NoError(t, err)
	return values
}
