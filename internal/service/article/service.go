package article

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"conduit/internal/gen/postgres"
	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	repo        repository
	users       UserService
	tags        TagService
	articleTags ArticleTagService
	favorites   FavoriteService
	follows     FollowService
}

func New(repo repository, users UserService, tags TagService, articleTags ArticleTagService, favorites FavoriteService, follows FollowService) *Service {
	return &Service{repo: repo, users: users, tags: tags, articleTags: articleTags, favorites: favorites, follows: follows}
}

func (s *Service) CreateArticle(ctx context.Context, req models.NewArticleRequest) (*models.SingleArticleResponse, error) {
	if err := validateCreate(req.Article); err != nil {
		return nil, err
	}
	authorID, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return nil, shared.ErrUnauthorized
	}
	slug := shared.GenerateSlug(req.Article.Title)
	err = s.repo.WithinTx(ctx, func(repo repository) error {
		created, createErr := repo.CreateArticle(ctx, postgres.CreateArticleParams{
			ID: shared.NewUUID(), AuthorID: shared.UUIDToPG(authorID), Slug: slug,
			Title: req.Article.Title, Description: req.Article.Description, Body: req.Article.Body,
		})
		if createErr != nil {
			return createErr
		}
		if req.Article.TagList == nil {
			return nil
		}
		return attachTags(ctx, repo, created, *req.Article.TagList)
	})
	if err != nil {
		return nil, err
	}
	return s.response(ctx, slug)
}

func (s *Service) UpdateArticle(ctx context.Context, slug string, req models.UpdateArticleRequest) (*models.SingleArticleResponse, error) {
	if err := validateUpdate(req.Article); err != nil {
		return nil, err
	}
	userID, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return nil, shared.ErrUnauthorized
	}
	err = s.repo.WithinTx(ctx, func(repo repository) error {
		current, ownerErr := requireOwner(ctx, repo, slug, userID)
		if ownerErr != nil {
			return ownerErr
		}
		updated, updateErr := repo.UpdateArticle(ctx, postgres.UpdateArticleParams{
			Slug: slug, Title: valueOr(req.Article.Title, current.Title),
			Description: valueOr(req.Article.Description, current.Description), Body: valueOr(req.Article.Body, current.Body),
		})
		if updateErr != nil {
			return updateErr
		}
		if req.Article.TagList == nil {
			return nil
		}
		if tagErr := repo.DeleteArticleTags(ctx, updated); tagErr != nil {
			return tagErr
		}
		return attachTags(ctx, repo, updated, *req.Article.TagList)
	})
	if err != nil {
		return nil, err
	}
	return s.response(ctx, slug)
}

func (s *Service) DeleteArticle(ctx context.Context, slug string) error {
	userID, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return shared.ErrUnauthorized
	}
	current, err := requireOwner(ctx, s.repo, slug, userID)
	if err != nil {
		return err
	}
	rows, err := s.repo.DeleteArticleBySlugAndAuthorID(ctx, postgres.DeleteArticleBySlugAndAuthorIDParams{Slug: slug, AuthorID: current.AuthorID})
	if err != nil {
		return err
	}
	if rows == 0 {
		return shared.NotFound("article")
	}
	return nil
}

func (s *Service) GetArticle(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	return s.response(ctx, slug)
}

func (s *Service) CreateArticleFavorite(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	return s.changeFavorite(ctx, slug, s.favorites.Add)
}

func (s *Service) DeleteArticleFavorite(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	return s.changeFavorite(ctx, slug, s.favorites.Remove)
}

func (s *Service) changeFavorite(ctx context.Context, slug string, change func(context.Context, uuid.UUID, uuid.UUID) error) (*models.SingleArticleResponse, error) {
	userID, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return nil, shared.ErrUnauthorized
	}
	articleID, err := s.repo.GetArticleIDBySlug(ctx, slug)
	if err != nil {
		return nil, mapArticleNotFound(err)
	}
	parsedArticleID, err := shared.PGToUUID(articleID)
	if err != nil {
		return nil, err
	}
	if err := change(ctx, parsedArticleID, userID); err != nil {
		return nil, err
	}
	return s.response(ctx, slug)
}

func (s *Service) GetArticles(ctx context.Context, params models.GetArticlesParams) (*models.MultipleArticlesResponse, error) {
	filters, err := s.filters(ctx, params)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return emptyResponse(), nil
		}
		return nil, err
	}
	return s.list(ctx, filters, pageLimit(params.Limit), pageOffset(params.Offset))
}

func (s *Service) GetArticlesFeed(ctx context.Context, params models.GetArticlesFeedParams) (*models.MultipleArticlesResponse, error) {
	viewerID, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return nil, shared.ErrUnauthorized
	}
	authorIDs, err := s.follows.FolloweeIDs(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	if len(authorIDs) == 0 {
		return emptyResponse(), nil
	}
	return s.list(ctx, articleFilters{authorIDs: authorIDs, filterAuthorIDs: true}, pageLimit(params.Limit), pageOffset(params.Offset))
}

func (s *Service) filters(ctx context.Context, params models.GetArticlesParams) (articleFilters, error) {
	var filters articleFilters
	if params.Author != nil {
		authorID, err := s.users.IDByUsername(ctx, *params.Author)
		if err != nil {
			return articleFilters{}, err
		}
		filters.authorIDs = []uuid.UUID{authorID}
		filters.filterAuthorIDs = true
	}
	if params.Tag != nil {
		tagID, err := s.tags.IDByName(ctx, *params.Tag)
		if err != nil {
			return articleFilters{}, err
		}
		articleIDs, err := s.articleTags.ArticleIDs(ctx, tagID)
		if err != nil {
			return articleFilters{}, err
		}
		filters.constrainArticles(articleIDs)
	}
	if params.Favorited != nil {
		userID, err := s.users.IDByUsername(ctx, *params.Favorited)
		if err != nil {
			return articleFilters{}, err
		}
		articleIDs, err := s.favorites.ArticleIDsForUser(ctx, userID)
		if err != nil {
			return articleFilters{}, err
		}
		filters.constrainArticles(articleIDs)
	}
	return filters, nil
}

func (s *Service) list(ctx context.Context, filters articleFilters, limit, offset int32) (*models.MultipleArticlesResponse, error) {
	if filters.filterArticleIDs && len(filters.articleIDs) == 0 {
		return emptyResponse(), nil
	}
	queryFilters := filters.queryParams()
	total, err := s.repo.CountArticles(ctx, queryFilters)
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return emptyResponse(), nil
	}
	rows, err := s.repo.ListArticles(ctx, postgres.ListArticlesParams{
		FilterAuthorIds: queryFilters.FilterAuthorIds, AuthorIds: queryFilters.AuthorIds,
		FilterArticleIds: queryFilters.FilterArticleIds, ArticleIds: queryFilters.ArticleIds,
		PageLimit: limit, PageOffset: offset,
	})
	if err != nil {
		return nil, err
	}
	articles, err := s.enrich(ctx, rows)
	if err != nil {
		return nil, err
	}
	return &models.MultipleArticlesResponse{Articles: articles, ArticlesCount: int(total)}, nil
}

func (s *Service) response(ctx context.Context, slug string) (*models.SingleArticleResponse, error) {
	row, err := s.repo.GetArticleBySlug(ctx, slug)
	if err != nil {
		return nil, mapArticleNotFound(err)
	}
	articles, err := s.enrich(ctx, []postgres.Article{row})
	if err != nil {
		return nil, err
	}
	return &models.SingleArticleResponse{Article: articles[0]}, nil
}

func (s *Service) enrich(ctx context.Context, rows []postgres.Article) ([]models.Article, error) {
	if len(rows) == 0 {
		return []models.Article{}, nil
	}
	articleIDs := make([]uuid.UUID, len(rows))
	authorIDs := make([]uuid.UUID, len(rows))
	for i := range rows {
		articleID, err := shared.PGToUUID(rows[i].ID)
		if err != nil {
			return nil, err
		}
		authorID, err := shared.PGToUUID(rows[i].AuthorID)
		if err != nil {
			return nil, err
		}
		articleIDs[i], authorIDs[i] = articleID, authorID
	}
	profiles, err := s.users.ProfilesByIDs(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	tagIDsByArticle, err := s.articleTags.TagIDsByArticleIDs(ctx, articleIDs)
	if err != nil {
		return nil, err
	}
	allTagIDs := make([]uuid.UUID, 0)
	for _, tagIDs := range tagIDsByArticle {
		allTagIDs = append(allTagIDs, tagIDs...)
	}
	tagNamesByID, err := s.tags.NamesByIDs(ctx, uniqueIDs(allTagIDs))
	if err != nil {
		return nil, err
	}
	favoriteCounts, err := s.favorites.CountsByArticleIDs(ctx, articleIDs)
	if err != nil {
		return nil, err
	}
	favorited := map[uuid.UUID]struct{}{}
	if viewerID, viewerErr := shared.UserIDFromContext(ctx); viewerErr == nil {
		favorited, err = s.favorites.FavoritedArticleIDs(ctx, viewerID, articleIDs)
		if err != nil {
			return nil, err
		}
	}
	articles := make([]models.Article, len(rows))
	for i := range rows {
		row := rows[i]
		articleID := articleIDs[i]
		authorID := authorIDs[i]
		profile, ok := profiles[authorID]
		if !ok {
			return nil, fmt.Errorf("profile for article author %s was not returned", authorID)
		}
		tagIDs := tagIDsByArticle[articleID]
		tagNames := make([]string, 0, len(tagIDs))
		for _, tagID := range tagIDs {
			if name, found := tagNamesByID[tagID]; found {
				tagNames = append(tagNames, name)
			}
		}
		slices.Sort(tagNames)
		_, isFavorited := favorited[articleID]
		articles[i] = models.Article{
			Author: profile, Body: row.Body, CreatedAt: row.CreatedAt.Time,
			Description: row.Description, Favorited: isFavorited, FavoritesCount: favoriteCounts[articleID],
			Slug: row.Slug, TagList: tagNames, Title: row.Title, UpdatedAt: row.UpdatedAt.Time,
		}
	}
	return articles, nil
}

type articleFilters struct {
	authorIDs        []uuid.UUID
	articleIDs       []uuid.UUID
	filterAuthorIDs  bool
	filterArticleIDs bool
}

func (f *articleFilters) constrainArticles(ids []uuid.UUID) {
	ids = uniqueIDs(ids)
	if !f.filterArticleIDs {
		f.articleIDs, f.filterArticleIDs = ids, true
		return
	}
	allowed := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		allowed[id] = struct{}{}
	}
	intersection := make([]uuid.UUID, 0, min(len(f.articleIDs), len(ids)))
	for _, id := range f.articleIDs {
		if _, ok := allowed[id]; ok {
			intersection = append(intersection, id)
		}
	}
	f.articleIDs = intersection
}

func (f articleFilters) queryParams() postgres.CountArticlesParams {
	return postgres.CountArticlesParams{
		FilterAuthorIds: f.filterAuthorIDs, AuthorIds: toPGUUIDs(f.authorIDs),
		FilterArticleIds: f.filterArticleIDs, ArticleIds: toPGUUIDs(f.articleIDs),
	}
}

func attachTags(ctx context.Context, repo repository, articleID pgtype.UUID, names []string) error {
	names = uniqueStrings(names)
	if len(names) == 0 {
		return nil
	}
	ids := make([]pgtype.UUID, len(names))
	for i := range names {
		ids[i] = shared.NewUUID()
	}
	tags, err := repo.UpsertTags(ctx, postgres.UpsertTagsParams{Ids: ids, Names: names})
	if err != nil {
		return err
	}
	tagIDs := make([]pgtype.UUID, len(tags))
	for i := range tags {
		tagIDs[i] = tags[i].ID
	}
	return repo.AttachTagsToArticle(ctx, postgres.AttachTagsToArticleParams{ArticleID: articleID, TagIds: tagIDs})
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func toPGUUIDs(ids []uuid.UUID) []pgtype.UUID {
	values := make([]pgtype.UUID, len(ids))
	for i := range ids {
		values[i] = shared.UUIDToPG(ids[i])
	}
	return values
}

func emptyResponse() *models.MultipleArticlesResponse {
	return &models.MultipleArticlesResponse{Articles: []models.Article{}}
}

func requireOwner(ctx context.Context, repo repository, slug string, userID uuid.UUID) (postgres.Article, error) {
	article, err := repo.GetArticleBySlug(ctx, slug)
	if err != nil {
		return postgres.Article{}, mapArticleNotFound(err)
	}
	authorID, err := shared.PGToUUID(article.AuthorID)
	if err != nil {
		return postgres.Article{}, err
	}
	if authorID != userID {
		return postgres.Article{}, shared.Forbidden("article")
	}
	return article, nil
}

func validateCreate(article models.NewArticle) error {
	for _, field := range []struct{ name, value string }{{"title", article.Title}, {"description", article.Description}, {"body", article.Body}} {
		if field.value == "" {
			return shared.Validation(field.name, "can't be blank")
		}
	}
	return nil
}

func validateUpdate(update models.UpdateArticle) error {
	for _, field := range []struct {
		name  string
		value *string
		set   bool
	}{{"title", update.Title, update.TitleSet}, {"description", update.Description, update.DescriptionSet}, {"body", update.Body, update.BodySet}} {
		if field.set && (field.value == nil || *field.value == "") {
			return shared.Validation(field.name, "can't be blank")
		}
	}
	if update.TagListSet && update.TagList == nil {
		return shared.Validation("tagList", "can't be null")
	}
	return nil
}

func mapArticleNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return shared.NotFound("article")
	}
	return err
}

func valueOr(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

func pageLimit(limit *int) int32 {
	if limit == nil || *limit < 0 || *limit > 100 {
		return 20
	}
	return int32(*limit)
}

func pageOffset(offset *int) int32 {
	if offset == nil || *offset < 0 {
		return 0
	}
	return int32(min(*offset, math.MaxInt32))
}
