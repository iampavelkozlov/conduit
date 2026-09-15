package user

import (
	"context"
	"errors"
	"fmt"

	"conduit/internal/gen/postgres"
	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	repo      repository
	follows   FollowService
	passwords PasswordManager
}

func New(repo repository, follows FollowService, passwords PasswordManager) *Service {
	return &Service{repo: repo, follows: follows, passwords: passwords}
}

func (s *Service) GetCurrentUser(ctx context.Context) (*models.UserResponse, error) {
	id, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return nil, shared.ErrUnauthorized
	}
	u, err := s.repo.GetUserByID(ctx, shared.UUIDToPG(id))
	if err != nil {
		return nil, mapNotFound(err)
	}
	response := toModelUser(&u)
	response.Token = shared.AccessTokenFromContext(ctx)
	return &models.UserResponse{User: response}, nil
}
func (s *Service) UpdateCurrentUser(ctx context.Context, req *models.UpdateUserRequest) (*models.UserResponse, error) {
	id, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return nil, shared.ErrUnauthorized
	}
	if req.User.EmailSet && (req.User.Email == nil || *req.User.Email == "") {
		return nil, shared.Validation("email", "can't be blank")
	}
	if req.User.UsernameSet && (req.User.Username == nil || *req.User.Username == "") {
		return nil, shared.Validation("username", "can't be blank")
	}
	if req.User.PasswordSet && req.User.Password == nil {
		return nil, shared.Validation("password", "can't be blank")
	}
	if req.User.PasswordSet && len(*req.User.Password) < 8 {
		return nil, shared.Validation("password", "is too short")
	}
	var hash string
	if req.User.PasswordSet {
		hash, err = s.passwords.Hash(*req.User.Password)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
	}
	u, err := s.repo.UpdateUser(ctx, postgres.UpdateUserParams{
		SetEmail: req.User.EmailSet, Email: shared.StringFromPtr(req.User.Email),
		SetUsername: req.User.UsernameSet, Username: shared.StringFromPtr(req.User.Username),
		SetPassword: req.User.PasswordSet, PasswordHash: hash,
		SetBio: req.User.BioSet, Bio: shared.TextFromPtr(req.User.Bio),
		SetImage: req.User.ImageSet, Image: shared.TextFromPtr(req.User.Image),
		ID: shared.UUIDToPG(id),
	})
	if err != nil {
		return nil, mapNotFound(err)
	}
	response := toModelUser(&u)
	response.Token = shared.AccessTokenFromContext(ctx)
	return &models.UserResponse{User: response}, nil
}
func (s *Service) FollowUserByUsername(ctx context.Context, username string) (*models.ProfileResponse, error) {
	viewer, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return nil, shared.ErrUnauthorized
	}
	target, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, mapNotFound(err)
	}
	targetID, err := shared.PGToUUID(target.ID)
	if err != nil {
		return nil, err
	}
	if followErr := s.follows.Follow(ctx, viewer, targetID); followErr != nil {
		return nil, followErr
	}
	profile, err := toProfile(&target, true)
	if err != nil {
		return nil, err
	}
	return &models.ProfileResponse{Profile: profile}, nil
}
func (s *Service) UnfollowUserByUsername(ctx context.Context, username string) (*models.ProfileResponse, error) {
	viewer, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return nil, shared.ErrUnauthorized
	}
	target, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, mapNotFound(err)
	}
	targetID, err := shared.PGToUUID(target.ID)
	if err != nil {
		return nil, err
	}
	if unfollowErr := s.follows.Unfollow(ctx, viewer, targetID); unfollowErr != nil {
		return nil, unfollowErr
	}
	profile, err := toProfile(&target, false)
	if err != nil {
		return nil, err
	}
	return &models.ProfileResponse{Profile: profile}, nil
}
func (s *Service) GetProfileByUsername(ctx context.Context, username string) (*models.ProfileResponse, error) {
	u, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return s.profile(ctx, &u)
}

func (s *Service) IDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	id, err := s.repo.GetUserIDByUsername(ctx, username)
	if err != nil {
		return uuid.Nil, mapNotFound(err)
	}
	return shared.PGToUUID(id)
}

func (s *Service) ProfilesByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]models.Profile, error) {
	profiles := make(map[uuid.UUID]models.Profile, len(ids))
	ids = uniqueIDs(ids)
	if len(ids) == 0 {
		return profiles, nil
	}
	values := make([]pgtype.UUID, len(ids))
	for i := range ids {
		values[i] = shared.UUIDToPG(ids[i])
	}
	users, err := s.repo.ListProfilesByIDs(ctx, values)
	if err != nil {
		return nil, err
	}
	following := map[uuid.UUID]struct{}{}
	if viewerID, viewerErr := shared.UserIDFromContext(ctx); viewerErr == nil {
		following, err = s.follows.FollowingIDs(ctx, viewerID, ids)
		if err != nil {
			return nil, err
		}
	}
	for i := range users {
		id, err := shared.PGToUUID(users[i].ID)
		if err != nil {
			return nil, err
		}
		_, isFollowing := following[id]
		profiles[id] = models.Profile{
			ID: id, Username: users[i].Username, Bio: users[i].Bio.String,
			Image: users[i].Image.String, Following: isFollowing,
		}
	}
	return profiles, nil
}
func (s *Service) profile(ctx context.Context, u *postgres.User) (*models.ProfileResponse, error) {
	target, err := shared.PGToUUID(u.ID)
	if err != nil {
		return nil, err
	}
	following := false
	if viewer, err := shared.UserIDFromContext(ctx); err == nil {
		following, err = s.follows.IsFollowing(ctx, viewer, target)
		if err != nil {
			return nil, err
		}
	}
	return &models.ProfileResponse{Profile: models.Profile{Bio: u.Bio.String, Following: following, ID: target, Image: u.Image.String, Username: u.Username}}, nil
}
func toModelUser(u *postgres.User) models.User {
	return models.User{Bio: u.Bio.String, Email: u.Email, Image: u.Image.String, Username: u.Username}
}
func toProfile(u *postgres.User, following bool) (models.Profile, error) {
	id, err := shared.PGToUUID(u.ID)
	if err != nil {
		return models.Profile{}, err
	}
	return models.Profile{Bio: u.Bio.String, Following: following, ID: id, Image: u.Image.String, Username: u.Username}, nil
}
func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return shared.NotFound("profile")
	}
	return err
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
