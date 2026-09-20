package posts

import (
	"context"
	"errors"
	"testing"
	"time"

	commonv1 "conduit/internal/gen/grpc/conduit/common/v1"
	postsv1 "conduit/internal/gen/grpc/conduit/posts/v1"
	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeArticles struct {
	getResponse    *models.SingleArticleResponse
	createResponse *models.SingleArticleResponse
	multiple       *models.MultipleArticlesResponse
	err            error
	created        models.NewArticleRequest
	createUser     uuid.UUID
	resolvedID     uuid.UUID
	resolveErr     error
}

func (f *fakeArticles) GetArticle(context.Context, string) (*models.SingleArticleResponse, error) {
	return f.getResponse, f.err
}

func (f *fakeArticles) CreateArticle(ctx context.Context, request models.NewArticleRequest) (*models.SingleArticleResponse, error) {
	f.created = request
	f.createUser, _ = shared.UserIDFromContext(ctx)
	return f.createResponse, f.err
}

func (f *fakeArticles) GetArticles(context.Context, models.GetArticlesParams) (*models.MultipleArticlesResponse, error) {
	return f.multiple, f.err
}

func (f *fakeArticles) GetArticlesFeed(context.Context, models.GetArticlesFeedParams) (*models.MultipleArticlesResponse, error) {
	return f.multiple, f.err
}

func (f *fakeArticles) UpdateArticle(context.Context, string, models.UpdateArticleRequest) (*models.SingleArticleResponse, error) {
	return f.getResponse, f.err
}

func (f *fakeArticles) DeleteArticle(context.Context, string) error {
	return f.err
}

func (f *fakeArticles) CreateArticleFavorite(context.Context, string) (*models.SingleArticleResponse, error) {
	return f.getResponse, f.err
}

func (f *fakeArticles) DeleteArticleFavorite(context.Context, string) (*models.SingleArticleResponse, error) {
	return f.getResponse, f.err
}

func (f *fakeArticles) ResolveArticleID(context.Context, string) (uuid.UUID, error) {
	return f.resolvedID, f.resolveErr
}

type fakeTags struct{ err error }

func (f fakeTags) GetTags(context.Context) (*models.TagsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &models.TagsResponse{Tags: []string{"go", "grpc"}}, nil
}

func TestServerCreateArticleMapsRequestAndResponse(t *testing.T) {
	t.Parallel()
	authorID, articleID := uuid.New(), uuid.New()
	now := time.Now().UTC()
	articles := &fakeArticles{createResponse: &models.SingleArticleResponse{Article: models.Article{
		ID: articleID, Author: models.Profile{ID: authorID}, Slug: "modern-go", Title: "Modern Go",
		Description: "description", Body: "body", TagList: []string{"go"}, CreatedAt: now, UpdatedAt: now,
	}}}
	server := NewServer(articles, fakeTags{})

	response, err := server.CreateArticle(t.Context(), &postsv1.CreateArticleRequest{
		AuthorId: authorID.String(), Title: "Modern Go", Description: "description", Body: "body", TagList: []string{"go"},
	})
	require.NoError(t, err)
	require.Equal(t, authorID, articles.createUser)
	require.Equal(t, []string{"go"}, *articles.created.Article.TagList)
	require.Equal(t, articleID.String(), response.GetArticle().GetId())
	require.Equal(t, authorID.String(), response.GetArticle().GetAuthorId())
}

func TestServerRejectsInvalidUserID(t *testing.T) {
	t.Parallel()
	server := NewServer(&fakeArticles{}, fakeTags{})
	_, err := server.CreateArticle(t.Context(), &postsv1.CreateArticleRequest{AuthorId: "bad"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestServerResolveAndListTags(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	server := NewServer(&fakeArticles{resolvedID: id}, fakeTags{})
	resolved, err := server.ResolveArticleID(t.Context(), &postsv1.ResolveArticleIDRequest{Slug: "article"})
	require.NoError(t, err)
	require.Equal(t, id.String(), resolved.GetArticleId())

	tags, err := server.ListTags(t.Context(), &postsv1.ListTagsRequest{})
	require.NoError(t, err)
	require.Equal(t, []string{"go", "grpc"}, tags.GetTags())
}

func TestServerResolveMissingArticle(t *testing.T) {
	t.Parallel()
	server := NewServer(&fakeArticles{resolveErr: shared.NotFound("article")}, fakeTags{})
	_, err := server.ResolveArticleID(t.Context(), &postsv1.ResolveArticleIDRequest{Slug: "missing"})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestServerArticleOperations(t *testing.T) {
	t.Parallel()
	userID, articleID := uuid.New(), uuid.New()
	article := models.Article{ID: articleID, Author: models.Profile{ID: userID}, Slug: "article", CreatedAt: time.Now(), UpdatedAt: time.Now(), FavoritesCount: -1}
	articles := &fakeArticles{
		getResponse: &models.SingleArticleResponse{Article: article},
		multiple:    &models.MultipleArticlesResponse{Articles: []models.Article{article}, ArticlesCount: 1},
	}
	server := NewServer(articles, fakeTags{})

	got, err := server.GetArticle(t.Context(), &postsv1.GetArticleRequest{Slug: "article", ViewerId: userID.String()})
	require.NoError(t, err)
	require.Zero(t, got.GetArticle().GetFavoritesCount())
	listed, err := server.ListArticles(t.Context(), &postsv1.ListArticlesRequest{
		ViewerId: userID.String(), Page: &commonv1.Page{Limit: 10, Offset: 2},
	})
	require.NoError(t, err)
	require.Len(t, listed.GetArticles(), 1)
	feed, err := server.ListFeed(t.Context(), &postsv1.ListFeedRequest{ViewerId: userID.String()})
	require.NoError(t, err)
	require.Equal(t, uint64(1), feed.GetArticlesCount())

	title := "updated"
	updated, err := server.UpdateArticle(t.Context(), &postsv1.UpdateArticleRequest{
		Slug: "article", RequesterId: userID.String(), Title: &title, TagList: &postsv1.TagList{Values: []string{"go"}},
	})
	require.NoError(t, err)
	require.Equal(t, articleID.String(), updated.GetArticle().GetId())
	deleted, err := server.DeleteArticle(t.Context(), &postsv1.DeleteArticleRequest{Slug: "article", RequesterId: userID.String()})
	require.NoError(t, err)
	require.True(t, deleted.GetDeleted())
	favorited, err := server.FavoriteArticle(t.Context(), &postsv1.FavoriteArticleRequest{Slug: "article", UserId: userID.String()})
	require.NoError(t, err)
	require.True(t, favorited.GetCreated())
	unfavorited, err := server.UnfavoriteArticle(t.Context(), &postsv1.UnfavoriteArticleRequest{Slug: "article", UserId: userID.String()})
	require.NoError(t, err)
	require.True(t, unfavorited.GetRemoved())
}

func TestServerMapsArticleErrors(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	articles := &fakeArticles{err: shared.Validation("article", "invalid"), resolveErr: errors.New("database unavailable")}
	server := NewServer(articles, fakeTags{})

	_, err := server.CreateArticle(t.Context(), &postsv1.CreateArticleRequest{AuthorId: userID.String(), Title: "article"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.GetArticle(t.Context(), &postsv1.GetArticleRequest{Slug: "article"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.ListArticles(t.Context(), &postsv1.ListArticlesRequest{})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.ListFeed(t.Context(), &postsv1.ListFeedRequest{ViewerId: userID.String()})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.UpdateArticle(t.Context(), &postsv1.UpdateArticleRequest{RequesterId: userID.String()})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.DeleteArticle(t.Context(), &postsv1.DeleteArticleRequest{RequesterId: userID.String()})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.FavoriteArticle(t.Context(), &postsv1.FavoriteArticleRequest{UserId: userID.String()})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.UnfavoriteArticle(t.Context(), &postsv1.UnfavoriteArticleRequest{UserId: userID.String()})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.ResolveArticleID(t.Context(), &postsv1.ResolveArticleIDRequest{Slug: "article"})
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestPageMapping(t *testing.T) {
	t.Parallel()
	require.Nil(t, pageLimit(nil))
	require.Nil(t, pageLimit(&commonv1.Page{}))
	require.Nil(t, pageOffset(nil))
	require.Equal(t, 7, *pageLimit(&commonv1.Page{Limit: 7}))
	require.Equal(t, 3, *pageOffset(&commonv1.Page{Offset: 3}))
}

func TestServerBoundaryValidationAndDependencyFailures(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	server := NewServer(&fakeArticles{resolveErr: shared.ErrInvalidUUID}, fakeTags{err: errors.New("tags")})

	_, err := server.GetArticle(t.Context(), &postsv1.GetArticleRequest{ViewerId: "bad"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.ListArticles(t.Context(), &postsv1.ListArticlesRequest{ViewerId: "bad"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.ListFeed(t.Context(), &postsv1.ListFeedRequest{})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.UpdateArticle(t.Context(), &postsv1.UpdateArticleRequest{})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.DeleteArticle(t.Context(), &postsv1.DeleteArticleRequest{})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.FavoriteArticle(t.Context(), &postsv1.FavoriteArticleRequest{})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.UnfavoriteArticle(t.Context(), &postsv1.UnfavoriteArticleRequest{UserId: "bad"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.ListTags(t.Context(), &postsv1.ListTagsRequest{})
	require.Equal(t, codes.Internal, status.Code(err))
	_, err = server.ResolveArticleID(t.Context(), &postsv1.ResolveArticleIDRequest{Slug: "bad-id"})
	require.Equal(t, codes.Internal, status.Code(err))

	ctx, err := withRequiredUser(t.Context(), userID.String(), "user_id")
	require.NoError(t, err)
	require.Equal(t, userID.String(), optionalUserID(ctx))
	require.Equal(t, uint64(7), nonnegativeUint64(7))
}
