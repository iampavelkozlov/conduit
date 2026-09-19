package profile

import (
	"context"
	"errors"
	"testing"

	commonv1 "conduit/internal/gen/grpc/conduit/common/v1"
	profilev1 "conduit/internal/gen/grpc/conduit/profile/v1"
	profiledomain "conduit/internal/service/profile"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type fakeProfileService struct {
	profile profiledomain.Profile
	update  profiledomain.Update
	err     error
}

func (f *fakeProfileService) Create(context.Context, uuid.UUID, string) (profiledomain.Profile, bool, error) {
	return f.profile, true, f.err
}

func (f *fakeProfileService) Get(context.Context, uuid.UUID) (profiledomain.Profile, error) {
	return f.profile, f.err
}

func (f *fakeProfileService) GetByUsername(context.Context, string) (profiledomain.Profile, error) {
	return f.profile, f.err
}

func (f *fakeProfileService) BatchGet(context.Context, []uuid.UUID) ([]profiledomain.Profile, error) {
	return []profiledomain.Profile{f.profile}, f.err
}

func (f *fakeProfileService) Update(_ context.Context, _ uuid.UUID, update profiledomain.Update) (profiledomain.Profile, error) {
	f.update = update
	return f.profile, f.err
}

func TestServerCreateAndBatch(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	bio := "bio"
	service := &fakeProfileService{profile: profiledomain.Profile{UserID: id, Username: "alice", Bio: &bio}}
	server := NewServer(service)

	created, err := server.CreateProfile(t.Context(), &profilev1.CreateProfileRequest{UserId: id.String(), Username: "alice"})
	require.NoError(t, err)
	require.True(t, created.GetCreated())
	require.Equal(t, bio, created.GetProfile().GetBio())

	batch, err := server.BatchGetProfiles(t.Context(), &profilev1.BatchGetProfilesRequest{UserIds: []string{id.String()}})
	require.NoError(t, err)
	require.Len(t, batch.GetProfiles(), 1)
}

func TestServerPreservesExplicitNull(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	service := &fakeProfileService{profile: profiledomain.Profile{UserID: id, Username: "alice"}}
	server := NewServer(service)

	_, err := server.UpdateProfile(t.Context(), &profilev1.UpdateProfileRequest{
		UserId: id.String(),
		Bio:    &commonv1.NullableString{Kind: &commonv1.NullableString_Null{Null: structpb.NullValue_NULL_VALUE}},
	})
	require.NoError(t, err)
	require.True(t, service.update.BioSet)
	require.Nil(t, service.update.Bio)
	require.False(t, service.update.ImageSet)
}

func TestServerRejectsMalformedIDs(t *testing.T) {
	t.Parallel()
	server := NewServer(&fakeProfileService{})
	_, err := server.CreateProfile(t.Context(), &profilev1.CreateProfileRequest{UserId: "bad"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.GetProfile(t.Context(), &profilev1.GetProfileRequest{UserId: "bad"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = server.BatchGetProfiles(t.Context(), &profilev1.BatchGetProfilesRequest{UserIds: []string{"bad"}})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.UpdateProfile(t.Context(), &profilev1.UpdateProfileRequest{UserId: "bad"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestServerGetAndUpdateValue(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	image := "https://example.test/image.png"
	service := &fakeProfileService{profile: profiledomain.Profile{UserID: id, Username: "alice", Image: &image}}
	server := NewServer(service)

	got, err := server.GetProfile(t.Context(), &profilev1.GetProfileRequest{UserId: id.String()})
	require.NoError(t, err)
	require.Equal(t, image, got.GetProfile().GetImage())
	byUsername, err := server.GetProfileByUsername(t.Context(), &profilev1.GetProfileByUsernameRequest{Username: "alice"})
	require.NoError(t, err)
	require.Equal(t, id.String(), byUsername.GetProfile().GetUserId())

	_, err = server.UpdateProfile(t.Context(), &profilev1.UpdateProfileRequest{
		UserId: id.String(), Image: &commonv1.NullableString{Kind: &commonv1.NullableString_Value{Value: image}},
	})
	require.NoError(t, err)
	require.True(t, service.update.ImageSet)
	require.Equal(t, image, *service.update.Image)
}

func TestServerMapsServiceErrors(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	server := NewServer(&fakeProfileService{err: errors.New("database unavailable")})

	_, err := server.CreateProfile(t.Context(), &profilev1.CreateProfileRequest{UserId: id.String(), Username: "alice"})
	require.Equal(t, codes.Internal, status.Code(err))
	_, err = server.GetProfile(t.Context(), &profilev1.GetProfileRequest{UserId: id.String()})
	require.Equal(t, codes.Internal, status.Code(err))
	_, err = server.GetProfileByUsername(t.Context(), &profilev1.GetProfileByUsernameRequest{Username: "alice"})
	require.Equal(t, codes.Internal, status.Code(err))
	_, err = server.BatchGetProfiles(t.Context(), &profilev1.BatchGetProfilesRequest{UserIds: []string{id.String()}})
	require.Equal(t, codes.Internal, status.Code(err))
	_, err = server.UpdateProfile(t.Context(), &profilev1.UpdateProfileRequest{UserId: id.String()})
	require.Equal(t, codes.Internal, status.Code(err))
}
