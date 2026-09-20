package subscriptions

import (
	"errors"
	"testing"

	subscriptionsv1 "conduit/internal/gen/grpc/conduit/subscriptions/v1"
	serviceshared "conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRegister(t *testing.T) {
	server := grpc.NewServer()
	Register(server, NewMockservice(gomock.NewController(t)))
	_, ok := server.GetServiceInfo()[subscriptionsv1.SubscriptionsService_ServiceDesc.ServiceName]
	require.True(t, ok)
}

func TestServerFollow(t *testing.T) {
	followerID, followeeID := uuid.New(), uuid.New()

	t.Run("success", func(t *testing.T) {
		service := NewMockservice(gomock.NewController(t))
		service.EXPECT().Follow(gomock.Any(), followerID, followeeID).Return(nil)
		response, err := NewServer(service).Follow(t.Context(), &subscriptionsv1.FollowRequest{
			FollowerId: followerID.String(), FolloweeId: followeeID.String(),
		})
		require.NoError(t, err)
		require.True(t, response.GetCreated())
	})

	t.Run("domain error", func(t *testing.T) {
		service := NewMockservice(gomock.NewController(t))
		service.EXPECT().Follow(gomock.Any(), followerID, followeeID).Return(serviceshared.ErrValidation)
		response, err := NewServer(service).Follow(t.Context(), &subscriptionsv1.FollowRequest{
			FollowerId: followerID.String(), FolloweeId: followeeID.String(),
		})
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestServerUnfollow(t *testing.T) {
	followerID, followeeID := uuid.New(), uuid.New()
	service := NewMockservice(gomock.NewController(t))
	service.EXPECT().Unfollow(gomock.Any(), followerID, followeeID).Return(nil)
	response, err := NewServer(service).Unfollow(t.Context(), &subscriptionsv1.UnfollowRequest{
		FollowerId: followerID.String(), FolloweeId: followeeID.String(),
	})
	require.NoError(t, err)
	require.True(t, response.GetRemoved())

	dependencyErr := errors.New("database offline")
	service.EXPECT().Unfollow(gomock.Any(), followerID, followeeID).Return(dependencyErr)
	response, err = NewServer(service).Unfollow(t.Context(), &subscriptionsv1.UnfollowRequest{
		FollowerId: followerID.String(), FolloweeId: followeeID.String(),
	})
	require.Nil(t, response)
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestServerIsFollowing(t *testing.T) {
	followerID, followeeID := uuid.New(), uuid.New()
	service := NewMockservice(gomock.NewController(t))
	service.EXPECT().IsFollowing(gomock.Any(), followerID, followeeID).Return(true, nil)
	response, err := NewServer(service).IsFollowing(t.Context(), &subscriptionsv1.IsFollowingRequest{
		FollowerId: followerID.String(), FolloweeId: followeeID.String(),
	})
	require.NoError(t, err)
	require.True(t, response.GetFollowing())

	service.EXPECT().IsFollowing(gomock.Any(), followerID, followeeID).Return(false, serviceshared.ErrNotFound)
	response, err = NewServer(service).IsFollowing(t.Context(), &subscriptionsv1.IsFollowingRequest{
		FollowerId: followerID.String(), FolloweeId: followeeID.String(),
	})
	require.Nil(t, response)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestServerListFolloweeIDs(t *testing.T) {
	followerID := uuid.New()
	want := []uuid.UUID{uuid.New(), uuid.New()}
	service := NewMockservice(gomock.NewController(t))
	service.EXPECT().FolloweeIDs(gomock.Any(), followerID).Return(want, nil)
	response, err := NewServer(service).ListFolloweeIDs(t.Context(), &subscriptionsv1.ListFolloweeIDsRequest{FollowerId: followerID.String()})
	require.NoError(t, err)
	require.Equal(t, []string{want[0].String(), want[1].String()}, response.GetFolloweeIds())

	service.EXPECT().FolloweeIDs(gomock.Any(), followerID).Return(nil, serviceshared.ErrForbidden)
	response, err = NewServer(service).ListFolloweeIDs(t.Context(), &subscriptionsv1.ListFolloweeIDsRequest{FollowerId: followerID.String()})
	require.Nil(t, response)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestServerListFollowingIDsPreservesOrderAndDeduplicates(t *testing.T) {
	followerID, first, second := uuid.New(), uuid.New(), uuid.New()
	candidates := []uuid.UUID{second, first, second, uuid.New()}
	service := NewMockservice(gomock.NewController(t))
	service.EXPECT().FollowingIDs(gomock.Any(), followerID, candidates).Return(map[uuid.UUID]struct{}{first: {}, second: {}}, nil)

	response, err := NewServer(service).ListFollowingIDs(t.Context(), &subscriptionsv1.ListFollowingIDsRequest{
		FollowerId: followerID.String(),
		CandidateIds: []string{
			second.String(), first.String(), second.String(), candidates[3].String(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{second.String(), first.String()}, response.GetFollowingIds())
}

func TestServerListFollowingIDsDependencyError(t *testing.T) {
	followerID, candidateID := uuid.New(), uuid.New()
	service := NewMockservice(gomock.NewController(t))
	service.EXPECT().FollowingIDs(gomock.Any(), followerID, []uuid.UUID{candidateID}).Return(nil, serviceshared.ErrUnauthorized)
	response, err := NewServer(service).ListFollowingIDs(t.Context(), &subscriptionsv1.ListFollowingIDsRequest{
		FollowerId: followerID.String(), CandidateIds: []string{candidateID.String()},
	})
	require.Nil(t, response)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestServerRejectsInvalidUUIDsBeforeCallingService(t *testing.T) {
	validID := uuid.New().String()
	server := NewServer(NewMockservice(gomock.NewController(t)))

	tests := map[string]func() error{
		"follow follower": func() error {
			_, err := server.Follow(t.Context(), &subscriptionsv1.FollowRequest{FollowerId: "invalid", FolloweeId: validID})
			return err
		},
		"follow followee": func() error {
			_, err := server.Follow(t.Context(), &subscriptionsv1.FollowRequest{FollowerId: validID, FolloweeId: "invalid"})
			return err
		},
		"unfollow": func() error {
			_, err := server.Unfollow(t.Context(), &subscriptionsv1.UnfollowRequest{})
			return err
		},
		"is following": func() error {
			_, err := server.IsFollowing(t.Context(), &subscriptionsv1.IsFollowingRequest{})
			return err
		},
		"list followees": func() error {
			_, err := server.ListFolloweeIDs(t.Context(), &subscriptionsv1.ListFolloweeIDsRequest{})
			return err
		},
		"list candidate": func() error {
			_, err := server.ListFollowingIDs(t.Context(), &subscriptionsv1.ListFollowingIDsRequest{FollowerId: validID, CandidateIds: []string{"invalid"}})
			return err
		},
		"list following follower": func() error {
			_, err := server.ListFollowingIDs(t.Context(), &subscriptionsv1.ListFollowingIDsRequest{FollowerId: "invalid"})
			return err
		},
	}
	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, codes.InvalidArgument, status.Code(call()))
		})
	}
}
