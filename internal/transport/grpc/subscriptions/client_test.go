package subscriptions

import (
	"context"
	"testing"

	subscriptionsv1 "conduit/internal/gen/grpc/conduit/subscriptions/v1"
	serviceshared "conduit/internal/service/shared"
	grpcshared "conduit/internal/transport/grpc/shared"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type fakeClient struct {
	followResponse   *subscriptionsv1.FollowResponse
	followErr        error
	unfollowErr      error
	isFollowing      bool
	followeeIDs      []string
	followingIDs     []string
	listFollowingReq *subscriptionsv1.ListFollowingIDsRequest
}

func (f *fakeClient) Follow(context.Context, *subscriptionsv1.FollowRequest, ...grpc.CallOption) (*subscriptionsv1.FollowResponse, error) {
	return f.followResponse, f.followErr
}
func (f *fakeClient) Unfollow(context.Context, *subscriptionsv1.UnfollowRequest, ...grpc.CallOption) (*subscriptionsv1.UnfollowResponse, error) {
	return &subscriptionsv1.UnfollowResponse{}, f.unfollowErr
}
func (f *fakeClient) IsFollowing(context.Context, *subscriptionsv1.IsFollowingRequest, ...grpc.CallOption) (*subscriptionsv1.IsFollowingResponse, error) {
	return &subscriptionsv1.IsFollowingResponse{Following: f.isFollowing}, nil
}
func (f *fakeClient) ListFolloweeIDs(context.Context, *subscriptionsv1.ListFolloweeIDsRequest, ...grpc.CallOption) (*subscriptionsv1.ListFolloweeIDsResponse, error) {
	return &subscriptionsv1.ListFolloweeIDsResponse{FolloweeIds: f.followeeIDs}, nil
}
func (f *fakeClient) ListFollowingIDs(_ context.Context, req *subscriptionsv1.ListFollowingIDsRequest, _ ...grpc.CallOption) (*subscriptionsv1.ListFollowingIDsResponse, error) {
	f.listFollowingReq = req
	return &subscriptionsv1.ListFollowingIDsResponse{FollowingIds: f.followingIDs}, nil
}

func TestClientOperations(t *testing.T) {
	t.Parallel()
	follower, followee := uuid.New(), uuid.New()
	remote := &fakeClient{followResponse: &subscriptionsv1.FollowResponse{Created: true}, isFollowing: true, followeeIDs: []string{followee.String()}, followingIDs: []string{followee.String()}}
	client := NewClient(remote)

	require.NoError(t, client.Follow(t.Context(), follower, followee))
	require.NoError(t, client.Unfollow(t.Context(), follower, followee))
	following, err := client.IsFollowing(t.Context(), follower, followee)
	require.NoError(t, err)
	require.True(t, following)
	ids, err := client.FolloweeIDs(t.Context(), follower)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{followee}, ids)
	matched, err := client.FollowingIDs(t.Context(), follower, []uuid.UUID{followee})
	require.NoError(t, err)
	require.Contains(t, matched, followee)
	require.Equal(t, []string{followee.String()}, remote.listFollowingReq.GetCandidateIds())
}

func TestClientMapsRemoteError(t *testing.T) {
	t.Parallel()
	client := NewClient(&fakeClient{followErr: grpcshared.EncodeError(serviceshared.Validation("profile", "can't follow yourself"))})
	err := client.Follow(t.Context(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, serviceshared.ErrValidation)
}

func TestClientRejectsMalformedResponseID(t *testing.T) {
	t.Parallel()
	client := NewClient(&fakeClient{followeeIDs: []string{"not-a-uuid"}})
	_, err := client.FolloweeIDs(t.Context(), uuid.New())
	require.Error(t, err)
}
