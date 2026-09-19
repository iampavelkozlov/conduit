package profile

import (
	"context"
	"errors"
	"fmt"

	commonv1 "conduit/internal/gen/grpc/conduit/common/v1"
	profilev1 "conduit/internal/gen/grpc/conduit/profile/v1"
	"conduit/internal/models"
	grpcshared "conduit/internal/transport/grpc/shared"

	"github.com/google/uuid"
)

type Update struct {
	Username    *string
	UsernameSet bool
	Bio         *string
	BioSet      bool
	Image       *string
	ImageSet    bool
}

type Snapshot struct {
	Profile models.Profile
	Bio     *string
	Image   *string
}

// Client implements comment.ProfileReader with one batch RPC.
type Client struct {
	client profilev1.ProfileServiceClient
}

func NewClient(client profilev1.ProfileServiceClient) *Client {
	return &Client{client: client}
}

func (c *Client) CreateProfile(ctx context.Context, userID uuid.UUID, username string) (models.Profile, error) {
	response, err := c.client.CreateProfile(ctx, &profilev1.CreateProfileRequest{UserId: userID.String(), Username: username})
	if err != nil {
		return models.Profile{}, grpcshared.DecodeError(err)
	}
	return parseProfile(response.GetProfile())
}

func (c *Client) GetProfile(ctx context.Context, userID uuid.UUID) (models.Profile, error) {
	snapshot, err := c.GetProfileSnapshot(ctx, userID)
	return snapshot.Profile, err
}

func (c *Client) GetProfileSnapshot(ctx context.Context, userID uuid.UUID) (Snapshot, error) {
	response, err := c.client.GetProfile(ctx, &profilev1.GetProfileRequest{UserId: userID.String()})
	if err != nil {
		return Snapshot{}, grpcshared.DecodeError(err)
	}
	profile, err := parseProfile(response.GetProfile())
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Profile: profile, Bio: cloneString(response.GetProfile().Bio), Image: cloneString(response.GetProfile().Image)}, nil
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	return new(*value)
}

func (c *Client) GetProfileByUsername(ctx context.Context, username string) (models.Profile, error) {
	response, err := c.client.GetProfileByUsername(ctx, &profilev1.GetProfileByUsernameRequest{Username: username})
	if err != nil {
		return models.Profile{}, grpcshared.DecodeError(err)
	}
	return parseProfile(response.GetProfile())
}

func (c *Client) UpdateProfile(ctx context.Context, userID uuid.UUID, update Update) (models.Profile, error) {
	request := &profilev1.UpdateProfileRequest{UserId: userID.String()}
	if update.UsernameSet {
		request.Username = update.Username
	}
	request.Bio = nullableString(update.Bio, update.BioSet)
	request.Image = nullableString(update.Image, update.ImageSet)
	response, err := c.client.UpdateProfile(ctx, request)
	if err != nil {
		return models.Profile{}, grpcshared.DecodeError(err)
	}
	return parseProfile(response.GetProfile())
}

// IDByUsername resolves legacy list filters. A UUID is accepted directly
// because the Posts gRPC contract already carries stable profile IDs.
func (c *Client) IDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	if id, err := uuid.Parse(username); err == nil {
		return id, nil
	}
	response, err := c.client.GetProfileByUsername(ctx, &profilev1.GetProfileByUsernameRequest{Username: username})
	if err != nil {
		return uuid.Nil, grpcshared.DecodeError(err)
	}
	if response.GetProfile() == nil {
		return uuid.Nil, errors.New("profile response is empty")
	}
	id, err := uuid.Parse(response.GetProfile().GetUserId())
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse profile user ID: %w", err)
	}
	return id, nil
}

func (c *Client) ProfilesByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]models.Profile, error) {
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = id.String()
	}
	response, err := c.client.BatchGetProfiles(ctx, &profilev1.BatchGetProfilesRequest{UserIds: values})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	profiles := make(map[uuid.UUID]models.Profile, len(response.GetProfiles()))
	for _, remote := range response.GetProfiles() {
		id, parseErr := uuid.Parse(remote.GetUserId())
		if parseErr != nil {
			return nil, fmt.Errorf("parse profile user ID: %w", parseErr)
		}
		profile, parseErr := parseProfile(remote)
		if parseErr != nil {
			return nil, parseErr
		}
		profiles[id] = profile
	}
	return profiles, nil
}

func parseProfile(remote *profilev1.Profile) (models.Profile, error) {
	if remote == nil {
		return models.Profile{}, errors.New("profile response is empty")
	}
	id, err := uuid.Parse(remote.GetUserId())
	if err != nil {
		return models.Profile{}, fmt.Errorf("parse profile user ID: %w", err)
	}
	return models.Profile{ID: id, Username: remote.GetUsername(), Bio: remote.GetBio(), Image: remote.GetImage()}, nil
}

func nullableString(value *string, set bool) *commonv1.NullableString {
	if !set {
		return nil
	}
	if value == nil {
		return &commonv1.NullableString{Kind: &commonv1.NullableString_Null{Null: 0}}
	}
	return &commonv1.NullableString{Kind: &commonv1.NullableString_Value{Value: *value}}
}
