package posts

import (
	"context"
	"errors"
	"fmt"
	"math"

	commonv1 "conduit/internal/gen/grpc/conduit/common/v1"
	postsv1 "conduit/internal/gen/grpc/conduit/posts/v1"
	"conduit/internal/models"
	"conduit/internal/service/shared"
	grpcshared "conduit/internal/transport/grpc/shared"

	"github.com/google/uuid"
)

// Client implements comment.ArticleResolver over the Posts gRPC API.
type Client struct {
	client        postsv1.PostsServiceClient
	profiles      profileReader
	subscriptions subscriptionReader
}

type profileReader interface {
	IDByUsername(context.Context, string) (uuid.UUID, error)
	ProfilesByIDs(context.Context, []uuid.UUID) (map[uuid.UUID]models.Profile, error)
}

type subscriptionReader interface {
	FolloweeIDs(context.Context, uuid.UUID) ([]uuid.UUID, error)
	FollowingIDs(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]struct{}, error)
}

func NewClient(client postsv1.PostsServiceClient) *Client {
	return &Client{client: client}
}

func NewApplicationClient(client postsv1.PostsServiceClient, profiles profileReader, subscriptions subscriptionReader) *Client {
	return &Client{client: client, profiles: profiles, subscriptions: subscriptions}
}

func (c *Client) ResolveArticleID(ctx context.Context, slug string) (uuid.UUID, error) {
	response, err := c.client.ResolveArticleID(ctx, &postsv1.ResolveArticleIDRequest{Slug: slug})
	if err != nil {
		return uuid.Nil, grpcshared.DecodeError(err)
	}
	id, err := uuid.Parse(response.GetArticleId())
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse posts article ID: %w", err)
	}
	return id, nil
}

func (c *Client) CreateArticle(ctx context.Context, request models.NewArticleRequest) (*models.SingleArticleResponse, error) {
	userID, err := requiredUserID(ctx)
	if err != nil {
		return nil, err
	}
	tags := []string{}
	if request.Article.TagList != nil {
		tags = *request.Article.TagList
	}
	response, err := c.client.CreateArticle(ctx, &postsv1.CreateArticleRequest{
		AuthorId: userID.String(), ViewerId: userID.String(), Title: request.Article.Title,
		Description: request.Article.Description, Body: request.Article.Body, TagList: tags,
	})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	article, err := c.hydrateOne(ctx, response.GetArticle())
	return &models.SingleArticleResponse{Article: article}, err
}

func (c *Client) GetArticle(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	response, err := c.client.GetArticle(ctx, &postsv1.GetArticleRequest{Slug: slug, ViewerId: optionalUserID(ctx)})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	article, err := c.hydrateOne(ctx, response.GetArticle())
	return &models.SingleArticleResponse{Article: article}, err
}

func (c *Client) UpdateArticle(ctx context.Context, slug string, request models.UpdateArticleRequest) (*models.SingleArticleResponse, error) {
	userID, err := requiredUserID(ctx)
	if err != nil {
		return nil, err
	}
	update := request.Article
	if (update.TitleSet && update.Title == nil) || (update.DescriptionSet && update.Description == nil) || (update.BodySet && update.Body == nil) {
		return nil, shared.ErrValidation
	}
	if update.TagListSet && update.TagList == nil {
		return nil, shared.Validation("tagList", "can't be null")
	}
	remote := &postsv1.UpdateArticleRequest{Slug: slug, RequesterId: userID.String(), ViewerId: userID.String()}
	if update.TitleSet {
		remote.Title = update.Title
	}
	if update.DescriptionSet {
		remote.Description = update.Description
	}
	if update.BodySet {
		remote.Body = update.Body
	}
	if update.TagListSet {
		remote.TagList = &postsv1.TagList{Values: *update.TagList}
	}
	response, err := c.client.UpdateArticle(ctx, remote)
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	article, err := c.hydrateOne(ctx, response.GetArticle())
	return &models.SingleArticleResponse{Article: article}, err
}

func (c *Client) DeleteArticle(ctx context.Context, slug string) error {
	userID, err := requiredUserID(ctx)
	if err != nil {
		return err
	}
	_, err = c.client.DeleteArticle(ctx, &postsv1.DeleteArticleRequest{Slug: slug, RequesterId: userID.String()})
	return grpcshared.DecodeError(err)
}

func (c *Client) GetArticles(ctx context.Context, params models.GetArticlesParams) (*models.MultipleArticlesResponse, error) {
	request := &postsv1.ListArticlesRequest{Tag: params.Tag, ViewerId: optionalUserID(ctx), Page: page(params.Limit, params.Offset)}
	var err error
	if params.Author != nil {
		id, resolveErr := c.profiles.IDByUsername(ctx, *params.Author)
		if resolveErr != nil {
			return nil, resolveErr
		}
		value := id.String()
		request.AuthorId = &value
	}
	if params.Favorited != nil {
		id, resolveErr := c.profiles.IDByUsername(ctx, *params.Favorited)
		if resolveErr != nil {
			return nil, resolveErr
		}
		value := id.String()
		request.FavoritedByUserId = &value
	}
	response, err := c.client.ListArticles(ctx, request)
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	articles, err := c.hydrate(ctx, response.GetArticles())
	if err != nil {
		return nil, err
	}
	count, err := safeInt(response.GetArticlesCount())
	if err != nil {
		return nil, err
	}
	return &models.MultipleArticlesResponse{Articles: articles, ArticlesCount: count}, nil
}

func (c *Client) GetArticlesFeed(ctx context.Context, params models.GetArticlesFeedParams) (*models.MultipleArticlesResponse, error) {
	userID, err := requiredUserID(ctx)
	if err != nil {
		return nil, err
	}
	authors, err := c.subscriptions.FolloweeIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	values := make([]string, len(authors))
	for i, id := range authors {
		values[i] = id.String()
	}
	response, err := c.client.ListFeed(ctx, &postsv1.ListFeedRequest{ViewerId: userID.String(), AuthorIds: values, Page: page(params.Limit, params.Offset)})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	articles, err := c.hydrate(ctx, response.GetArticles())
	if err != nil {
		return nil, err
	}
	count, err := safeInt(response.GetArticlesCount())
	if err != nil {
		return nil, err
	}
	return &models.MultipleArticlesResponse{Articles: articles, ArticlesCount: count}, nil
}

func (c *Client) CreateArticleFavorite(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	return c.favorite(ctx, slug, true)
}

func (c *Client) DeleteArticleFavorite(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	return c.favorite(ctx, slug, false)
}

func (c *Client) favorite(ctx context.Context, slug string, create bool) (*models.SingleArticleResponse, error) {
	userID, err := requiredUserID(ctx)
	if err != nil {
		return nil, err
	}
	var remote *postsv1.Article
	if create {
		response, callErr := c.client.FavoriteArticle(ctx, &postsv1.FavoriteArticleRequest{Slug: slug, UserId: userID.String()})
		if callErr != nil {
			return nil, grpcshared.DecodeError(callErr)
		}
		remote = response.GetArticle()
	} else {
		response, callErr := c.client.UnfavoriteArticle(ctx, &postsv1.UnfavoriteArticleRequest{Slug: slug, UserId: userID.String()})
		if callErr != nil {
			return nil, grpcshared.DecodeError(callErr)
		}
		remote = response.GetArticle()
	}
	article, err := c.hydrateOne(ctx, remote)
	return &models.SingleArticleResponse{Article: article}, err
}

func (c *Client) GetTags(ctx context.Context) (*models.TagsResponse, error) {
	response, err := c.client.ListTags(ctx, &postsv1.ListTagsRequest{})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	return &models.TagsResponse{Tags: response.GetTags()}, nil
}

func (c *Client) hydrateOne(ctx context.Context, remote *postsv1.Article) (models.Article, error) {
	values, err := c.hydrate(ctx, []*postsv1.Article{remote})
	if err != nil {
		return models.Article{}, err
	}
	return values[0], nil
}

func (c *Client) hydrate(ctx context.Context, remote []*postsv1.Article) ([]models.Article, error) {
	if len(remote) == 0 {
		return []models.Article{}, nil
	}
	authorIDs := make([]uuid.UUID, 0, len(remote))
	parsed := make([]uuid.UUID, len(remote))
	seen := make(map[uuid.UUID]struct{})
	for i, article := range remote {
		if article == nil {
			return nil, errors.New("posts response contains a nil article")
		}
		id, err := uuid.Parse(article.GetAuthorId())
		if err != nil {
			return nil, fmt.Errorf("parse article author ID: %w", err)
		}
		parsed[i] = id
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			authorIDs = append(authorIDs, id)
		}
	}
	profiles, err := c.profiles.ProfilesByIDs(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	if viewerID, viewerErr := shared.UserIDFromContext(ctx); viewerErr == nil && c.subscriptions != nil {
		following, followErr := c.subscriptions.FollowingIDs(ctx, viewerID, authorIDs)
		if followErr != nil {
			return nil, followErr
		}
		for id := range following {
			profile := profiles[id]
			profile.Following = true
			profiles[id] = profile
		}
	}
	result := make([]models.Article, len(remote))
	for i, article := range remote {
		profile, ok := profiles[parsed[i]]
		if !ok {
			return nil, fmt.Errorf("profile for article author %s was not returned", parsed[i])
		}
		id, err := uuid.Parse(article.GetId())
		if err != nil {
			return nil, fmt.Errorf("parse article ID: %w", err)
		}
		favoritesCount, countErr := safeInt(article.GetFavoritesCount())
		if countErr != nil {
			return nil, countErr
		}
		tags := article.GetTagList()
		if tags == nil {
			tags = []string{}
		}
		result[i] = models.Article{ID: id, Slug: article.GetSlug(), Title: article.GetTitle(), Description: article.GetDescription(), Body: article.GetBody(), TagList: tags, Author: profile, CreatedAt: article.GetCreatedAt().AsTime(), UpdatedAt: article.GetUpdatedAt().AsTime(), Favorited: article.GetFavorited(), FavoritesCount: favoritesCount}
	}
	return result, nil
}

func safeInt(value uint64) (int, error) {
	if value > uint64(math.MaxInt) {
		return 0, fmt.Errorf("value %d overflows int", value)
	}
	return int(value), nil
}

func requiredUserID(ctx context.Context) (uuid.UUID, error) {
	id, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return uuid.Nil, shared.ErrUnauthorized
	}
	return id, nil
}

func optionalUserID(ctx context.Context) string {
	id, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return ""
	}
	return id.String()
}

func page(limit, offset *int) *commonv1.Page {
	l, o := 20, 0
	if limit != nil && *limit >= 0 && *limit <= 100 {
		l = *limit
	}
	if offset != nil && *offset >= 0 {
		o = *offset
	}
	return &commonv1.Page{Limit: uint32(l), Offset: uint32(o)}
}
