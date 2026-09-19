package posts

import (
	"context"
	"strconv"

	commonv1 "conduit/internal/gen/grpc/conduit/common/v1"
	postsv1 "conduit/internal/gen/grpc/conduit/posts/v1"
	"conduit/internal/models"
	"conduit/internal/service/shared"
	grpcshared "conduit/internal/transport/grpc/shared"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type articleService interface {
	CreateArticle(context.Context, models.NewArticleRequest) (*models.SingleArticleResponse, error)
	GetArticle(context.Context, string) (*models.SingleArticleResponse, error)
	GetArticles(context.Context, models.GetArticlesParams) (*models.MultipleArticlesResponse, error)
	GetArticlesFeed(context.Context, models.GetArticlesFeedParams) (*models.MultipleArticlesResponse, error)
	UpdateArticle(context.Context, string, models.UpdateArticleRequest) (*models.SingleArticleResponse, error)
	DeleteArticle(context.Context, string) error
	CreateArticleFavorite(context.Context, string) (*models.SingleArticleResponse, error)
	DeleteArticleFavorite(context.Context, string) (*models.SingleArticleResponse, error)
	ResolveArticleID(context.Context, string) (uuid.UUID, error)
}

type tagService interface {
	GetTags(context.Context) (*models.TagsResponse, error)
}

type Server struct {
	postsv1.UnimplementedPostsServiceServer
	articles articleService
	tags     tagService
}

func NewServer(articles articleService, tags tagService) *Server {
	return &Server{articles: articles, tags: tags}
}

func (s *Server) CreateArticle(ctx context.Context, request *postsv1.CreateArticleRequest) (*postsv1.CreateArticleResponse, error) {
	ctx, err := withRequiredUser(ctx, request.GetAuthorId(), "author_id")
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	tags := request.GetTagList()
	response, err := s.articles.CreateArticle(ctx, models.NewArticleRequest{Article: models.NewArticle{
		Title: request.GetTitle(), Description: request.GetDescription(), Body: request.GetBody(), TagList: &tags,
	}})
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &postsv1.CreateArticleResponse{Article: toProto(&response.Article)}, nil
}

func (s *Server) GetArticle(ctx context.Context, request *postsv1.GetArticleRequest) (*postsv1.GetArticleResponse, error) {
	ctx, err := withOptionalUser(ctx, request.GetViewerId(), "viewer_id")
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	response, err := s.articles.GetArticle(ctx, request.GetSlug())
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &postsv1.GetArticleResponse{Article: toProto(&response.Article)}, nil
}

func (s *Server) ResolveArticleID(ctx context.Context, request *postsv1.ResolveArticleIDRequest) (*postsv1.ResolveArticleIDResponse, error) {
	id, err := s.articles.ResolveArticleID(ctx, request.GetSlug())
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &postsv1.ResolveArticleIDResponse{ArticleId: id.String()}, nil
}

func (s *Server) ListArticles(ctx context.Context, request *postsv1.ListArticlesRequest) (*postsv1.ListArticlesResponse, error) {
	ctx, err := withOptionalUser(ctx, request.GetViewerId(), "viewer_id")
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	params := models.GetArticlesParams{Limit: pageLimit(request.GetPage()), Offset: pageOffset(request.GetPage())}
	params.Tag = request.Tag
	params.Author = request.AuthorId
	params.Favorited = request.FavoritedByUserId
	response, err := s.articles.GetArticles(ctx, params)
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &postsv1.ListArticlesResponse{Articles: toProtos(response.Articles), ArticlesCount: nonnegativeUint64(response.ArticlesCount)}, nil
}

func (s *Server) ListFeed(ctx context.Context, request *postsv1.ListFeedRequest) (*postsv1.ListFeedResponse, error) {
	ctx, err := withRequiredUser(ctx, request.GetViewerId(), "viewer_id")
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	response, err := s.articles.GetArticlesFeed(ctx, models.GetArticlesFeedParams{
		Limit: pageLimit(request.GetPage()), Offset: pageOffset(request.GetPage()),
	})
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &postsv1.ListFeedResponse{Articles: toProtos(response.Articles), ArticlesCount: nonnegativeUint64(response.ArticlesCount)}, nil
}

func (s *Server) UpdateArticle(ctx context.Context, request *postsv1.UpdateArticleRequest) (*postsv1.UpdateArticleResponse, error) {
	ctx, err := withRequiredUser(ctx, request.GetRequesterId(), "requester_id")
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	update := models.UpdateArticle{
		Title: request.Title, TitleSet: request.Title != nil,
		Description: request.Description, DescriptionSet: request.Description != nil,
		Body: request.Body, BodySet: request.Body != nil,
		TagListSet: request.TagList != nil,
	}
	if request.TagList != nil {
		values := request.TagList.GetValues()
		update.TagList = &values
	}
	response, err := s.articles.UpdateArticle(ctx, request.GetSlug(), models.UpdateArticleRequest{Article: update})
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &postsv1.UpdateArticleResponse{Article: toProto(&response.Article)}, nil
}

func (s *Server) DeleteArticle(ctx context.Context, request *postsv1.DeleteArticleRequest) (*postsv1.DeleteArticleResponse, error) {
	ctx, err := withRequiredUser(ctx, request.GetRequesterId(), "requester_id")
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	if err := s.articles.DeleteArticle(ctx, request.GetSlug()); err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &postsv1.DeleteArticleResponse{Deleted: true}, nil
}

func (s *Server) FavoriteArticle(ctx context.Context, request *postsv1.FavoriteArticleRequest) (*postsv1.FavoriteArticleResponse, error) {
	ctx, err := withRequiredUser(ctx, request.GetUserId(), "user_id")
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	response, err := s.articles.CreateArticleFavorite(ctx, request.GetSlug())
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &postsv1.FavoriteArticleResponse{Article: toProto(&response.Article), Created: true}, nil
}

func (s *Server) UnfavoriteArticle(ctx context.Context, request *postsv1.UnfavoriteArticleRequest) (*postsv1.UnfavoriteArticleResponse, error) {
	ctx, err := withRequiredUser(ctx, request.GetUserId(), "user_id")
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	response, err := s.articles.DeleteArticleFavorite(ctx, request.GetSlug())
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &postsv1.UnfavoriteArticleResponse{Article: toProto(&response.Article), Removed: true}, nil
}

func (s *Server) ListTags(ctx context.Context, _ *postsv1.ListTagsRequest) (*postsv1.ListTagsResponse, error) {
	response, err := s.tags.GetTags(ctx)
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &postsv1.ListTagsResponse{Tags: response.Tags}, nil
}

func withRequiredUser(ctx context.Context, value, field string) (context.Context, error) {
	if value == "" {
		return ctx, shared.Validation(field, "must not be empty")
	}
	return withOptionalUser(ctx, value, field)
}

func withOptionalUser(ctx context.Context, value, field string) (context.Context, error) {
	if value == "" {
		return ctx, nil
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return ctx, shared.Validation(field, "must be a valid UUID")
	}
	return shared.WithUserID(ctx, id), nil
}

func pageLimit(page *commonv1.Page) *int {
	if page == nil || page.GetLimit() == 0 {
		return nil
	}
	value := int(page.GetLimit())
	return &value
}

func pageOffset(page *commonv1.Page) *int {
	if page == nil {
		return nil
	}
	value := int(page.GetOffset())
	return &value
}

func toProtos(articles []models.Article) []*postsv1.Article {
	result := make([]*postsv1.Article, len(articles))
	for i := range articles {
		result[i] = toProto(&articles[i])
	}
	return result
}

func toProto(article *models.Article) *postsv1.Article {
	return &postsv1.Article{
		Id: article.ID.String(), Slug: article.Slug, Title: article.Title,
		Description: article.Description, Body: article.Body, TagList: article.TagList,
		AuthorId: article.Author.ID.String(), CreatedAt: timestamppb.New(article.CreatedAt),
		UpdatedAt: timestamppb.New(article.UpdatedAt), Favorited: article.Favorited,
		FavoritesCount: nonnegativeUint64(article.FavoritesCount),
	}
}

func nonnegativeUint64(value int) uint64 {
	if value <= 0 {
		return 0
	}
	parsed, _ := strconv.ParseUint(strconv.Itoa(value), 10, 64)
	return parsed
}

var _ postsv1.PostsServiceServer = (*Server)(nil)
