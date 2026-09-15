package http

import (
	"context"
	"time"

	api "conduit/internal/gen/http"
	"conduit/internal/models"
)

func articleListDTO(article *models.Article) struct {
	Author         api.Profile `json:"author"`
	CreatedAt      time.Time   `json:"createdAt"`
	Description    string      `json:"description"`
	Favorited      bool        `json:"favorited"`
	FavoritesCount int         `json:"favoritesCount"`
	Slug           string      `json:"slug"`
	TagList        []string    `json:"tagList"`
	Title          string      `json:"title"`
	UpdatedAt      time.Time   `json:"updatedAt"`
} {
	return struct {
		Author         api.Profile `json:"author"`
		CreatedAt      time.Time   `json:"createdAt"`
		Description    string      `json:"description"`
		Favorited      bool        `json:"favorited"`
		FavoritesCount int         `json:"favoritesCount"`
		Slug           string      `json:"slug"`
		TagList        []string    `json:"tagList"`
		Title          string      `json:"title"`
		UpdatedAt      time.Time   `json:"updatedAt"`
	}{Author: profileDTO(article.Author), CreatedAt: article.CreatedAt, Description: article.Description, Favorited: article.Favorited, FavoritesCount: article.FavoritesCount, Slug: article.Slug, TagList: article.TagList, Title: article.Title, UpdatedAt: article.UpdatedAt}
}

func (s Server) CreateArticle(ctx context.Context, request api.CreateArticleRequestObject) (api.CreateArticleResponseObject, error) {
	dto := request.Body.Article
	model := models.NewArticleRequest{Article: models.NewArticle{Body: dto.Body, Description: dto.Description, TagList: dto.TagList, Title: dto.Title}}
	resp, err := s.svc.CreateArticle(ctx, model)
	if err != nil {
		return nil, err
	}
	return api.CreateArticle201JSONResponse{SingleArticleResponseJSONResponse: api.SingleArticleResponseJSONResponse{Article: articleDTO(&resp.Article)}}, nil
}

func (s Server) UpdateArticle(ctx context.Context, request api.UpdateArticleRequestObject) (api.UpdateArticleResponseObject, error) {
	dto := request.Body.Article
	model := models.UpdateArticleRequest{Article: models.UpdateArticle{
		Body: dto.Body, BodySet: updateFieldWasSet(ctx, "article", "body"),
		Description: dto.Description, DescriptionSet: updateFieldWasSet(ctx, "article", "description"),
		TagList: dto.TagList, TagListSet: updateFieldWasSet(ctx, "article", "tagList"),
		Title: dto.Title, TitleSet: updateFieldWasSet(ctx, "article", "title"),
	}}
	resp, err := s.svc.UpdateArticle(ctx, request.Slug, model)
	if err != nil {
		return nil, err
	}
	return api.UpdateArticle200JSONResponse{SingleArticleResponseJSONResponse: api.SingleArticleResponseJSONResponse{Article: articleDTO(&resp.Article)}}, nil
}

func (s Server) CreateArticleFavorite(ctx context.Context, request api.CreateArticleFavoriteRequestObject) (api.CreateArticleFavoriteResponseObject, error) {
	resp, err := s.svc.CreateArticleFavorite(ctx, request.Slug)
	if err != nil {
		return nil, err
	}
	return api.CreateArticleFavorite200JSONResponse{SingleArticleResponseJSONResponse: api.SingleArticleResponseJSONResponse{Article: articleDTO(&resp.Article)}}, nil
}

func (s Server) DeleteArticle(ctx context.Context, request api.DeleteArticleRequestObject) (api.DeleteArticleResponseObject, error) {
	if err := s.svc.DeleteArticle(ctx, request.Slug); err != nil {
		return nil, err
	}
	return api.DeleteArticle204Response{}, nil
}

func (s Server) DeleteArticleFavorite(ctx context.Context, request api.DeleteArticleFavoriteRequestObject) (api.DeleteArticleFavoriteResponseObject, error) {
	resp, err := s.svc.DeleteArticleFavorite(ctx, request.Slug)
	if err != nil {
		return nil, err
	}
	return api.DeleteArticleFavorite200JSONResponse{SingleArticleResponseJSONResponse: api.SingleArticleResponseJSONResponse{Article: articleDTO(&resp.Article)}}, nil
}

func (s Server) GetArticle(ctx context.Context, request api.GetArticleRequestObject) (api.GetArticleResponseObject, error) {
	resp, err := s.svc.GetArticle(ctx, request.Slug)
	if err != nil {
		return nil, err
	}
	return api.GetArticle200JSONResponse{SingleArticleResponseJSONResponse: api.SingleArticleResponseJSONResponse{Article: articleDTO(&resp.Article)}}, nil
}

func (s Server) GetArticles(ctx context.Context, request api.GetArticlesRequestObject) (api.GetArticlesResponseObject, error) {
	dto := request.Params
	params := models.GetArticlesParams{Tag: dto.Tag, Author: dto.Author, Favorited: dto.Favorited, Limit: dto.Limit, Offset: dto.Offset}
	resp, err := s.svc.GetArticles(ctx, params)
	if err != nil {
		return nil, err
	}
	articles := make([]struct {
		Author         api.Profile `json:"author"`
		CreatedAt      time.Time   `json:"createdAt"`
		Description    string      `json:"description"`
		Favorited      bool        `json:"favorited"`
		FavoritesCount int         `json:"favoritesCount"`
		Slug           string      `json:"slug"`
		TagList        []string    `json:"tagList"`
		Title          string      `json:"title"`
		UpdatedAt      time.Time   `json:"updatedAt"`
	}, len(resp.Articles))
	for i := range resp.Articles {
		articles[i] = articleListDTO(&resp.Articles[i])
	}
	return api.GetArticles200JSONResponse{MultipleArticlesResponseJSONResponse: api.MultipleArticlesResponseJSONResponse{Articles: articles, ArticlesCount: resp.ArticlesCount}}, nil
}

func (s Server) GetArticlesFeed(ctx context.Context, request api.GetArticlesFeedRequestObject) (api.GetArticlesFeedResponseObject, error) {
	dto := request.Params
	resp, err := s.svc.GetArticlesFeed(ctx, models.GetArticlesFeedParams{Limit: dto.Limit, Offset: dto.Offset})
	if err != nil {
		return nil, err
	}
	articles := make([]struct {
		Author         api.Profile `json:"author"`
		CreatedAt      time.Time   `json:"createdAt"`
		Description    string      `json:"description"`
		Favorited      bool        `json:"favorited"`
		FavoritesCount int         `json:"favoritesCount"`
		Slug           string      `json:"slug"`
		TagList        []string    `json:"tagList"`
		Title          string      `json:"title"`
		UpdatedAt      time.Time   `json:"updatedAt"`
	}, len(resp.Articles))
	for i := range resp.Articles {
		articles[i] = articleListDTO(&resp.Articles[i])
	}
	return api.GetArticlesFeed200JSONResponse{MultipleArticlesResponseJSONResponse: api.MultipleArticlesResponseJSONResponse{Articles: articles, ArticlesCount: resp.ArticlesCount}}, nil
}
