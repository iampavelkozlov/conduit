package subscriptions

import (
	"context"

	subscriptionsv1 "conduit/internal/gen/grpc/conduit/subscriptions/v1"
	grpcshared "conduit/internal/transport/grpc/shared"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//go:generate go run go.uber.org/mock/mockgen -source=server.go -destination=mock_service_test.go -package=subscriptions
type service interface {
	Follow(context.Context, uuid.UUID, uuid.UUID) error
	Unfollow(context.Context, uuid.UUID, uuid.UUID) error
	IsFollowing(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	FolloweeIDs(context.Context, uuid.UUID) ([]uuid.UUID, error)
	FollowingIDs(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]struct{}, error)
}

type Server struct {
	subscriptionsv1.UnimplementedSubscriptionsServiceServer
	service service
}

func NewServer(service service) *Server {
	return &Server{service: service}
}

func Register(registrar grpc.ServiceRegistrar, service service) {
	subscriptionsv1.RegisterSubscriptionsServiceServer(registrar, NewServer(service))
}

func (s *Server) Follow(ctx context.Context, request *subscriptionsv1.FollowRequest) (*subscriptionsv1.FollowResponse, error) {
	followerID, followeeID, err := parsePair(request.GetFollowerId(), request.GetFolloweeId())
	if err != nil {
		return nil, err
	}
	if err := s.service.Follow(ctx, followerID, followeeID); err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &subscriptionsv1.FollowResponse{Created: true}, nil
}

func (s *Server) Unfollow(ctx context.Context, request *subscriptionsv1.UnfollowRequest) (*subscriptionsv1.UnfollowResponse, error) {
	followerID, followeeID, err := parsePair(request.GetFollowerId(), request.GetFolloweeId())
	if err != nil {
		return nil, err
	}
	if err := s.service.Unfollow(ctx, followerID, followeeID); err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &subscriptionsv1.UnfollowResponse{Removed: true}, nil
}

func (s *Server) IsFollowing(ctx context.Context, request *subscriptionsv1.IsFollowingRequest) (*subscriptionsv1.IsFollowingResponse, error) {
	followerID, followeeID, err := parsePair(request.GetFollowerId(), request.GetFolloweeId())
	if err != nil {
		return nil, err
	}
	following, err := s.service.IsFollowing(ctx, followerID, followeeID)
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &subscriptionsv1.IsFollowingResponse{Following: following}, nil
}

func (s *Server) ListFolloweeIDs(ctx context.Context, request *subscriptionsv1.ListFolloweeIDsRequest) (*subscriptionsv1.ListFolloweeIDsResponse, error) {
	followerID, err := parseUUID("follower_id", request.GetFollowerId())
	if err != nil {
		return nil, err
	}
	ids, err := s.service.FolloweeIDs(ctx, followerID)
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &subscriptionsv1.ListFolloweeIDsResponse{FolloweeIds: stringifyUUIDs(ids)}, nil
}

func (s *Server) ListFollowingIDs(ctx context.Context, request *subscriptionsv1.ListFollowingIDsRequest) (*subscriptionsv1.ListFollowingIDsResponse, error) {
	followerID, err := parseUUID("follower_id", request.GetFollowerId())
	if err != nil {
		return nil, err
	}
	candidates := make([]uuid.UUID, len(request.GetCandidateIds()))
	for i, value := range request.GetCandidateIds() {
		candidates[i], err = parseUUID("candidate_ids", value)
		if err != nil {
			return nil, err
		}
	}
	following, err := s.service.FollowingIDs(ctx, followerID, candidates)
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}

	// Preserve request order and de-duplicate the response. This keeps the RPC
	// deterministic without relying on Go map iteration order.
	ids := make([]string, 0, len(following))
	seen := make(map[uuid.UUID]struct{}, len(following))
	for _, candidateID := range candidates {
		if _, ok := following[candidateID]; !ok {
			continue
		}
		if _, ok := seen[candidateID]; ok {
			continue
		}
		seen[candidateID] = struct{}{}
		ids = append(ids, candidateID.String())
	}
	return &subscriptionsv1.ListFollowingIDsResponse{FollowingIds: ids}, nil
}

func parsePair(follower, followee string) (uuid.UUID, uuid.UUID, error) {
	followerID, err := parseUUID("follower_id", follower)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	followeeID, err := parseUUID("followee_id", followee)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return followerID, followeeID, nil
}

func parseUUID(field, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s must be a UUID", field)
	}
	return id, nil
}

func stringifyUUIDs(ids []uuid.UUID) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = id.String()
	}
	return result
}
