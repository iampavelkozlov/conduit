package favorite

import (
	"context"

	"conduit/internal/gen/postgres"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service owns article_favorites and exposes its operations as domain methods.
type Service struct{ repo repository }

func New(repo repository) *Service { return &Service{repo: repo} }
func (s *Service) Add(ctx context.Context, articleID, userID uuid.UUID) error {
	return s.repo.FavoriteArticle(ctx, postgres.FavoriteArticleParams{ArticleID: shared.UUIDToPG(articleID), UserID: shared.UUIDToPG(userID)})
}
func (s *Service) Remove(ctx context.Context, articleID, userID uuid.UUID) error {
	return s.repo.UnfavoriteArticle(ctx, postgres.UnfavoriteArticleParams{ArticleID: shared.UUIDToPG(articleID), UserID: shared.UUIDToPG(userID)})
}
func (s *Service) ArticleIDsForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := s.repo.ListFavoriteArticleIDsByUserID(ctx, shared.UUIDToPG(userID))
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		parsed, err := shared.PGToUUID(id)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

func (s *Service) CountsByArticleIDs(ctx context.Context, articleIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	counts := make(map[uuid.UUID]int, len(articleIDs))
	if len(articleIDs) == 0 {
		return counts, nil
	}
	rows, err := s.repo.CountFavoritesByArticleIDs(ctx, toPGUUIDs(articleIDs))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		articleID, err := shared.PGToUUID(row.ArticleID)
		if err != nil {
			return nil, err
		}
		counts[articleID] = int(row.FavoritesCount)
	}
	return counts, nil
}

func (s *Service) FavoritedArticleIDs(ctx context.Context, userID uuid.UUID, articleIDs []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	result := make(map[uuid.UUID]struct{})
	if len(articleIDs) == 0 {
		return result, nil
	}
	ids, err := s.repo.ListFavoriteArticleIDsByUserIDAndArticleIDs(ctx, postgres.ListFavoriteArticleIDsByUserIDAndArticleIDsParams{
		UserID:     shared.UUIDToPG(userID),
		ArticleIds: toPGUUIDs(articleIDs),
	})
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		articleID, err := shared.PGToUUID(id)
		if err != nil {
			return nil, err
		}
		result[articleID] = struct{}{}
	}
	return result, nil
}

func toPGUUIDs(ids []uuid.UUID) []pgtype.UUID {
	values := make([]pgtype.UUID, len(ids))
	for i := range ids {
		values[i] = shared.UUIDToPG(ids[i])
	}
	return values
}
