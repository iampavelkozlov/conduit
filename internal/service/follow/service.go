package follow

import (
	"context"
	"log/slog"

	"conduit/internal/gen/postgres"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service owns the follows table. It deliberately knows nothing about users.
type Service struct {
	repo   repository
	logger *slog.Logger
}

func New(repo repository, loggers ...*slog.Logger) *Service {
	return &Service{repo: repo, logger: shared.ServiceLogger(loggers...)}
}

func (s *Service) Follow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	if followerID == followeeID {
		return shared.ErrValidation
	}
	err := s.repo.FollowUser(ctx, postgres.FollowUserParams{FollowerID: shared.UUIDToPG(followerID), FolloweeID: shared.UUIDToPG(followeeID)})
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "follow repo err", shared.ErrorAttrs(err, slog.String("follower_id", followerID.String()), slog.String("followee_id", followeeID.String()))...)
	}
	return err
}

func (s *Service) Unfollow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	err := s.repo.UnfollowUser(ctx, postgres.UnfollowUserParams{FollowerID: shared.UUIDToPG(followerID), FolloweeID: shared.UUIDToPG(followeeID)})
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "follow repo err", shared.ErrorAttrs(err, slog.String("follower_id", followerID.String()), slog.String("followee_id", followeeID.String()))...)
	}
	return err
}

func (s *Service) IsFollowing(ctx context.Context, followerID, followeeID uuid.UUID) (bool, error) {
	following, err := s.repo.IsFollowing(ctx, postgres.IsFollowingParams{FollowerID: shared.UUIDToPG(followerID), FolloweeID: shared.UUIDToPG(followeeID)})
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "follow repo err", shared.ErrorAttrs(err, slog.String("follower_id", followerID.String()), slog.String("followee_id", followeeID.String()))...)
	}
	return following, err
}

func (s *Service) FolloweeIDs(ctx context.Context, followerID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := s.repo.ListFolloweeIDsByFollowerID(ctx, shared.UUIDToPG(followerID))
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "follow repo err", shared.ErrorAttrs(err, slog.String("follower_id", followerID.String()))...)
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		parsed, err := shared.PGToUUID(id)
		if err != nil {
			s.logger.LogAttrs(ctx, slog.LevelError, "follow uuid err", shared.ErrorAttrs(err, slog.String("follower_id", followerID.String()), shared.UUIDAttr("followee_id", id))...)
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

func (s *Service) FollowingIDs(ctx context.Context, followerID uuid.UUID, candidateIDs []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	result := make(map[uuid.UUID]struct{})
	if len(candidateIDs) == 0 {
		return result, nil
	}
	values := make([]pgtype.UUID, len(candidateIDs))
	for i := range candidateIDs {
		values[i] = shared.UUIDToPG(candidateIDs[i])
	}
	ids, err := s.repo.ListFollowingIDs(ctx, postgres.ListFollowingIDsParams{
		FollowerID:   shared.UUIDToPG(followerID),
		CandidateIds: values,
	})
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "follow repo err", shared.ErrorAttrs(err, slog.String("follower_id", followerID.String()), slog.Any("candidate_ids", candidateIDs))...)
		return nil, err
	}
	for _, id := range ids {
		parsed, err := shared.PGToUUID(id)
		if err != nil {
			s.logger.LogAttrs(ctx, slog.LevelError, "follow uuid err", shared.ErrorAttrs(err, slog.String("follower_id", followerID.String()), shared.UUIDAttr("followee_id", id))...)
			return nil, err
		}
		result[parsed] = struct{}{}
	}
	return result, nil
}
