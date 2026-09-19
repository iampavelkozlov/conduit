package subscriptions

import (
	"context"
	"fmt"

	subscriptionsv1 "conduit/internal/gen/grpc/conduit/subscriptions/v1"
	grpcshared "conduit/internal/transport/grpc/shared"

	"github.com/google/uuid"
)

// Client adapts the subscriptions gRPC API to the narrow follow contracts used
// by the profile and posts application services.
type Client struct {
	client subscriptionsv1.SubscriptionsServiceClient
}

func NewClient(client subscriptionsv1.SubscriptionsServiceClient) *Client {
	return &Client{client: client}
}

func (c *Client) Follow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	_, err := c.client.Follow(ctx, &subscriptionsv1.FollowRequest{
		FollowerId: followerID.String(), FolloweeId: followeeID.String(),
	})
	return grpcshared.DecodeError(err)
}

func (c *Client) Unfollow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	_, err := c.client.Unfollow(ctx, &subscriptionsv1.UnfollowRequest{
		FollowerId: followerID.String(), FolloweeId: followeeID.String(),
	})
	return grpcshared.DecodeError(err)
}

func (c *Client) IsFollowing(ctx context.Context, followerID, followeeID uuid.UUID) (bool, error) {
	response, err := c.client.IsFollowing(ctx, &subscriptionsv1.IsFollowingRequest{
		FollowerId: followerID.String(), FolloweeId: followeeID.String(),
	})
	if err != nil {
		return false, grpcshared.DecodeError(err)
	}
	return response.GetFollowing(), nil
}

func (c *Client) FolloweeIDs(ctx context.Context, followerID uuid.UUID) ([]uuid.UUID, error) {
	response, err := c.client.ListFolloweeIDs(ctx, &subscriptionsv1.ListFolloweeIDsRequest{FollowerId: followerID.String()})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	return parseIDs(response.GetFolloweeIds())
}

func (c *Client) FollowingIDs(ctx context.Context, followerID uuid.UUID, candidateIDs []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	candidates := make([]string, len(candidateIDs))
	for i, id := range candidateIDs {
		candidates[i] = id.String()
	}
	response, err := c.client.ListFollowingIDs(ctx, &subscriptionsv1.ListFollowingIDsRequest{
		FollowerId: followerID.String(), CandidateIds: candidates,
	})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	ids, err := parseIDs(response.GetFollowingIds())
	if err != nil {
		return nil, err
	}
	result := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result, nil
}

func parseIDs(values []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, len(values))
	for i, value := range values {
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, fmt.Errorf("parse subscriptions response UUID: %w", err)
		}
		ids[i] = id
	}
	return ids, nil
}
