package profile

import (
	"context"
	"testing"

	profilev1 "conduit/internal/gen/grpc/conduit/profile/v1"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeProfileClient struct {
	profilev1.ProfileServiceClient
	byUsername  *profilev1.Profile
	profiles    []*profilev1.Profile
	created     *profilev1.Profile
	got         *profilev1.Profile
	updated     *profilev1.Profile
	updateReq   *profilev1.UpdateProfileRequest
	createErr   error
	getErr      error
	usernameErr error
	batchErr    error
	updateErr   error
}

func (f *fakeProfileClient) CreateProfile(_ context.Context, _ *profilev1.CreateProfileRequest, _ ...grpc.CallOption) (*profilev1.CreateProfileResponse, error) {
	return &profilev1.CreateProfileResponse{Profile: f.created}, f.createErr
}

func (f *fakeProfileClient) GetProfile(_ context.Context, _ *profilev1.GetProfileRequest, _ ...grpc.CallOption) (*profilev1.GetProfileResponse, error) {
	return &profilev1.GetProfileResponse{Profile: f.got}, f.getErr
}

func (f *fakeProfileClient) UpdateProfile(_ context.Context, request *profilev1.UpdateProfileRequest, _ ...grpc.CallOption) (*profilev1.UpdateProfileResponse, error) {
	f.updateReq = request
	return &profilev1.UpdateProfileResponse{Profile: f.updated}, f.updateErr
}

func (f *fakeProfileClient) GetProfileByUsername(context.Context, *profilev1.GetProfileByUsernameRequest, ...grpc.CallOption) (*profilev1.GetProfileByUsernameResponse, error) {
	return &profilev1.GetProfileByUsernameResponse{Profile: f.byUsername}, f.usernameErr
}

func TestClientCreateGetAndUpdate(t *testing.T) {
	id := uuid.New()
	bio := "bio"
	remoteProfile := &profilev1.Profile{UserId: id.String(), Username: "alice", Bio: &bio}
	remote := &fakeProfileClient{created: remoteProfile, got: remoteProfile, updated: remoteProfile}
	client := NewClient(remote)

	created, err := client.CreateProfile(t.Context(), id, "alice")
	require.NoError(t, err)
	require.Equal(t, "alice", created.Username)
	got, err := client.GetProfile(t.Context(), id)
	require.NoError(t, err)
	require.Equal(t, "bio", got.Bio)
	username := "alice"
	updated, err := client.UpdateProfile(t.Context(), id, Update{Username: &username, UsernameSet: true, BioSet: true, ImageSet: true})
	require.NoError(t, err)
	require.Equal(t, id, updated.ID)
	require.NotNil(t, remote.updateReq.GetBio())
	require.NotNil(t, remote.updateReq.GetImage())
	require.Equal(t, username, remote.updateReq.GetUsername())
}

func (f *fakeProfileClient) BatchGetProfiles(context.Context, *profilev1.BatchGetProfilesRequest, ...grpc.CallOption) (*profilev1.BatchGetProfilesResponse, error) {
	return &profilev1.BatchGetProfilesResponse{Profiles: f.profiles}, f.batchErr
}

func TestClientRemoteAndMalformedResponses(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	remoteErr := status.Error(codes.Unavailable, "unavailable")

	tests := []struct {
		name string
		call func(*Client) error
		fake *fakeProfileClient
	}{
		{name: "create remote", fake: &fakeProfileClient{createErr: remoteErr}, call: func(client *Client) error { _, err := client.CreateProfile(t.Context(), id, "alice"); return err }},
		{name: "create malformed", fake: &fakeProfileClient{created: &profilev1.Profile{UserId: "bad"}}, call: func(client *Client) error { _, err := client.CreateProfile(t.Context(), id, "alice"); return err }},
		{name: "get remote", fake: &fakeProfileClient{getErr: remoteErr}, call: func(client *Client) error { _, err := client.GetProfileSnapshot(t.Context(), id); return err }},
		{name: "get malformed", fake: &fakeProfileClient{got: &profilev1.Profile{UserId: "bad"}}, call: func(client *Client) error { _, err := client.GetProfileSnapshot(t.Context(), id); return err }},
		{name: "username remote", fake: &fakeProfileClient{usernameErr: remoteErr}, call: func(client *Client) error { _, err := client.GetProfileByUsername(t.Context(), "alice"); return err }},
		{name: "username malformed", fake: &fakeProfileClient{byUsername: &profilev1.Profile{UserId: "bad"}}, call: func(client *Client) error { _, err := client.GetProfileByUsername(t.Context(), "alice"); return err }},
		{name: "update remote", fake: &fakeProfileClient{updateErr: remoteErr}, call: func(client *Client) error { _, err := client.UpdateProfile(t.Context(), id, Update{}); return err }},
		{name: "update malformed", fake: &fakeProfileClient{updated: &profilev1.Profile{UserId: "bad"}}, call: func(client *Client) error { _, err := client.UpdateProfile(t.Context(), id, Update{}); return err }},
		{name: "batch remote", fake: &fakeProfileClient{batchErr: remoteErr}, call: func(client *Client) error { _, err := client.ProfilesByIDs(t.Context(), []uuid.UUID{id}); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Error(t, test.call(NewClient(test.fake)))
		})
	}
}

func TestClientUsernameResolutionFailures(t *testing.T) {
	t.Parallel()
	remoteErr := status.Error(codes.Unavailable, "unavailable")
	_, err := NewClient(&fakeProfileClient{usernameErr: remoteErr}).IDByUsername(t.Context(), "alice")
	require.Error(t, err)
	_, err = NewClient(&fakeProfileClient{}).IDByUsername(t.Context(), "alice")
	require.ErrorContains(t, err, "empty")
	_, err = NewClient(&fakeProfileClient{byUsername: &profilev1.Profile{UserId: "bad"}}).IDByUsername(t.Context(), "alice")
	require.ErrorContains(t, err, "parse profile user ID")
}

func TestCloneString(t *testing.T) {
	t.Parallel()
	require.Nil(t, cloneString(nil))
	value := "value"
	clone := cloneString(&value)
	require.Equal(t, value, *clone)
	require.NotSame(t, &value, clone)
}

func TestClientProfiles(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	bio := "bio"
	remote := &fakeProfileClient{
		byUsername: &profilev1.Profile{UserId: id.String(), Username: "alice"},
		profiles:   []*profilev1.Profile{{UserId: id.String(), Username: "alice", Bio: &bio}},
	}
	client := NewClient(remote)

	resolved, err := client.IDByUsername(t.Context(), "alice")
	require.NoError(t, err)
	require.Equal(t, id, resolved)
	resolved, err = client.IDByUsername(t.Context(), id.String())
	require.NoError(t, err)
	require.Equal(t, id, resolved)

	profiles, err := client.ProfilesByIDs(t.Context(), []uuid.UUID{id})
	require.NoError(t, err)
	require.Equal(t, "alice", profiles[id].Username)
	require.Equal(t, "bio", profiles[id].Bio)
}

func TestClientRejectsMalformedProfileID(t *testing.T) {
	t.Parallel()
	client := NewClient(&fakeProfileClient{profiles: []*profilev1.Profile{{UserId: "bad"}}})
	_, err := client.ProfilesByIDs(t.Context(), []uuid.UUID{uuid.New()})
	require.ErrorContains(t, err, "parse profile user ID")
}

func TestNullableStringAndProfileValidation(t *testing.T) {
	require.Nil(t, nullableString(nil, false))
	require.NotNil(t, nullableString(nil, true).GetKind())
	value := "value"
	require.Equal(t, value, nullableString(&value, true).GetValue())
	_, err := parseProfile(nil)
	require.ErrorContains(t, err, "empty")
	_, err = parseProfile(&profilev1.Profile{UserId: "invalid"})
	require.ErrorContains(t, err, "user ID")
}
