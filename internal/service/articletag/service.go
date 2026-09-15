package articletag

import (
	"context"

	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service owns read operations for the article_tags relation table.
type Service struct{ repo repository }

func New(repo repository) *Service { return &Service{repo: repo} }

func (s *Service) ArticleIDs(ctx context.Context, tagID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := s.repo.ListArticleIDsByTagID(ctx, shared.UUIDToPG(tagID))
	if err != nil {
		return nil, err
	}
	return parseUUIDs(ids)
}

func (s *Service) TagIDsByArticleIDs(ctx context.Context, articleIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	result := make(map[uuid.UUID][]uuid.UUID, len(articleIDs))
	if len(articleIDs) == 0 {
		return result, nil
	}
	rows, err := s.repo.ListArticleTagRelationsByArticleIDs(ctx, toPGUUIDs(articleIDs))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		articleID, err := shared.PGToUUID(row.ArticleID)
		if err != nil {
			return nil, err
		}
		tagID, err := shared.PGToUUID(row.TagID)
		if err != nil {
			return nil, err
		}
		result[articleID] = append(result[articleID], tagID)
	}
	return result, nil
}

func parseUUIDs(values []pgtype.UUID) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		id, err := shared.PGToUUID(value)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func toPGUUIDs(ids []uuid.UUID) []pgtype.UUID {
	values := make([]pgtype.UUID, len(ids))
	for i := range ids {
		values[i] = shared.UUIDToPG(ids[i])
	}
	return values
}
