package profile

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	profilepostgres "conduit/internal/gen/profile/postgres"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type Profile struct {
	UserID   uuid.UUID
	Username string
	Bio      *string
	Image    *string
}

type Update struct {
	Username *string
	Bio      *string
	BioSet   bool
	Image    *string
	ImageSet bool
}

type repository interface {
	CreateProfile(context.Context, profilepostgres.CreateProfileParams) (profilepostgres.Profile, error)
	GetProfileByID(context.Context, pgtype.UUID) (profilepostgres.Profile, error)
	GetProfileByUsername(context.Context, string) (profilepostgres.Profile, error)
	ListProfilesByIDs(context.Context, []pgtype.UUID) ([]profilepostgres.Profile, error)
	UpdateProfile(context.Context, profilepostgres.UpdateProfileParams) (profilepostgres.Profile, error)
}

type cache interface {
	GetByID(context.Context, uuid.UUID) (Profile, bool, error)
	GetByUsername(context.Context, string) (Profile, bool, error)
	Set(context.Context, Profile) error
	Delete(context.Context, uuid.UUID, ...string) error
}

// Cache is the storage-facing profile cache port.
type Cache = cache

type Service struct {
	repo   repository
	cache  cache
	logger *slog.Logger
}

func New(repo repository, cache cache, logger *slog.Logger) *Service {
	return &Service{repo: repo, cache: cache, logger: shared.ServiceLogger(logger)}
}

func (s *Service) Create(ctx context.Context, id uuid.UUID, username string) (Profile, bool, error) {
	if strings.TrimSpace(username) == "" {
		return Profile{}, false, shared.Validation("username", "can't be blank")
	}
	row, err := s.repo.CreateProfile(ctx, profilepostgres.CreateProfileParams{UserID: shared.UUIDToPG(id), Username: username})
	created := true
	if errors.Is(err, pgx.ErrNoRows) {
		created = false
		row, err = s.repo.GetProfileByID(ctx, shared.UUIDToPG(id))
	}
	if err != nil {
		return Profile{}, false, s.mapRepositoryError(ctx, err, id)
	}
	profile, err := fromRow(&row)
	if err != nil {
		return Profile{}, false, err
	}
	s.setCache(ctx, profile)
	return profile, created, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Profile, error) {
	if profile, found := s.cachedByID(ctx, id); found {
		return profile, nil
	}
	row, err := s.repo.GetProfileByID(ctx, shared.UUIDToPG(id))
	if err != nil {
		return Profile{}, s.mapRepositoryError(ctx, err, id)
	}
	profile, err := fromRow(&row)
	if err != nil {
		return Profile{}, err
	}
	s.setCache(ctx, profile)
	return profile, nil
}

func (s *Service) GetByUsername(ctx context.Context, username string) (Profile, error) {
	if profile, found := s.cachedByUsername(ctx, username); found {
		return profile, nil
	}
	row, err := s.repo.GetProfileByUsername(ctx, username)
	if err != nil {
		return Profile{}, s.mapRepositoryError(ctx, err, uuid.Nil)
	}
	profile, err := fromRow(&row)
	if err != nil {
		return Profile{}, err
	}
	s.setCache(ctx, profile)
	return profile, nil
}

func (s *Service) BatchGet(ctx context.Context, ids []uuid.UUID) ([]Profile, error) {
	ids = uniqueIDs(ids)
	profiles := make(map[uuid.UUID]Profile, len(ids))
	missing := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if profile, found := s.cachedByID(ctx, id); found {
			profiles[id] = profile
		} else {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		values := make([]pgtype.UUID, len(missing))
		for i, id := range missing {
			values[i] = shared.UUIDToPG(id)
		}
		rows, err := s.repo.ListProfilesByIDs(ctx, values)
		if err != nil {
			return nil, s.mapRepositoryError(ctx, err, uuid.Nil)
		}
		for i := range rows {
			profile, err := fromRow(&rows[i])
			if err != nil {
				return nil, err
			}
			profiles[profile.UserID] = profile
			s.setCache(ctx, profile)
		}
	}
	result := make([]Profile, 0, len(profiles))
	for _, id := range ids {
		if profile, ok := profiles[id]; ok {
			result = append(result, profile)
		}
	}
	return result, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, update Update) (Profile, error) {
	if update.Username != nil && strings.TrimSpace(*update.Username) == "" {
		return Profile{}, shared.Validation("username", "can't be blank")
	}
	current, err := s.repo.GetProfileByID(ctx, shared.UUIDToPG(id))
	if err != nil {
		return Profile{}, s.mapRepositoryError(ctx, err, id)
	}
	row, err := s.repo.UpdateProfile(ctx, profilepostgres.UpdateProfileParams{
		SetUsername: update.Username != nil, Username: stringValue(update.Username),
		SetBio: update.BioSet, Bio: textValue(update.Bio), SetImage: update.ImageSet,
		Image: textValue(update.Image), UserID: shared.UUIDToPG(id),
	})
	if err != nil {
		return Profile{}, s.mapRepositoryError(ctx, err, id)
	}
	profile, err := fromRow(&row)
	if err != nil {
		return Profile{}, err
	}
	s.invalidate(ctx, id, current.Username, profile.Username)
	s.setCache(ctx, profile)
	return profile, nil
}

func (s *Service) cachedByID(ctx context.Context, id uuid.UUID) (Profile, bool) {
	profile, found, err := s.cache.GetByID(ctx, id)
	if err != nil {
		s.logger.WarnContext(ctx, "profile cache read failed", "user_id", id.String(), "error", err)
		return Profile{}, false
	}
	return profile, found
}

func (s *Service) cachedByUsername(ctx context.Context, username string) (Profile, bool) {
	profile, found, err := s.cache.GetByUsername(ctx, username)
	if err != nil {
		s.logger.WarnContext(ctx, "profile cache read failed", "username", username, "error", err)
		return Profile{}, false
	}
	return profile, found
}

func (s *Service) setCache(ctx context.Context, profile Profile) {
	if err := s.cache.Set(ctx, profile); err != nil {
		s.logger.WarnContext(ctx, "profile cache write failed", "user_id", profile.UserID.String(), "error", err)
	}
}

func (s *Service) invalidate(ctx context.Context, id uuid.UUID, usernames ...string) {
	if err := s.cache.Delete(ctx, id, usernames...); err != nil {
		s.logger.WarnContext(ctx, "profile cache invalidation failed", "user_id", id.String(), "error", err)
	}
}

func (s *Service) mapRepositoryError(ctx context.Context, err error, id uuid.UUID) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return shared.NotFound("profile")
	}
	if postgresError, ok := errors.AsType[*pgconn.PgError](err); ok && postgresError.Code == "23505" {
		return shared.Validation("username", "has already been taken")
	}
	s.logger.ErrorContext(ctx, "profile repository failed", "user_id", id.String(), "error", err)
	return err
}

func fromRow(row *profilepostgres.Profile) (Profile, error) {
	id, err := shared.PGToUUID(row.UserID)
	if err != nil {
		return Profile{}, err
	}
	return Profile{UserID: id, Username: row.Username, Bio: textPointer(row.Bio), Image: textPointer(row.Image)}, nil
}

func textPointer(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func textValue(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
