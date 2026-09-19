package comments

import (
	"context"
	"errors"
	"fmt"
	"math"

	commentsv1 "conduit/internal/gen/grpc/conduit/comments/v1"
	"conduit/internal/models"
	"conduit/internal/service/shared"
	grpcshared "conduit/internal/transport/grpc/shared"

	"github.com/google/uuid"
)

type articleResolver interface {
	ResolveArticleID(context.Context, string) (uuid.UUID, error)
}

type profileReader interface {
	ProfilesByIDs(context.Context, []uuid.UUID) (map[uuid.UUID]models.Profile, error)
}

type subscriptionReader interface {
	FollowingIDs(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]struct{}, error)
}

// Client adapts Comments gRPC to the unchanged HTTP-facing comment contract.
type Client struct {
	client        commentsv1.CommentsServiceClient
	articles      articleResolver
	profiles      profileReader
	subscriptions subscriptionReader
}

func NewApplicationClient(client commentsv1.CommentsServiceClient, articles articleResolver, profiles profileReader, subscriptions subscriptionReader) *Client {
	return &Client{client: client, articles: articles, profiles: profiles, subscriptions: subscriptions}
}

func NewClient(client commentsv1.CommentsServiceClient, articles articleResolver, profiles profileReader) *Client {
	return &Client{client: client, articles: articles, profiles: profiles}
}

func (c *Client) CreateArticleComment(ctx context.Context, slug string, request models.NewCommentRequest) (*models.SingleCommentResponse, error) {
	if request.Comment.Body == "" {
		return nil, shared.Validation("body", "can't be blank")
	}
	authorID, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return nil, shared.ErrUnauthorized
	}
	articleID, err := c.articles.ResolveArticleID(ctx, slug)
	if err != nil {
		return nil, err
	}
	response, err := c.client.CreateComment(ctx, &commentsv1.CreateCommentRequest{
		ArticleId: articleID.String(), AuthorId: authorID.String(), Body: request.Comment.Body,
	})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	comments, err := c.hydrate(ctx, []*commentsv1.Comment{response.GetComment()})
	if err != nil {
		return nil, err
	}
	return &models.SingleCommentResponse{Comment: comments[0]}, nil
}

func (c *Client) GetArticleComments(ctx context.Context, slug string) (*models.MultipleCommentsResponse, error) {
	articleID, err := c.articles.ResolveArticleID(ctx, slug)
	if err != nil {
		return nil, err
	}
	response, err := c.client.ListComments(ctx, &commentsv1.ListCommentsRequest{ArticleId: articleID.String()})
	if err != nil {
		return nil, grpcshared.DecodeError(err)
	}
	comments, err := c.hydrate(ctx, response.GetComments())
	if err != nil {
		return nil, err
	}
	return &models.MultipleCommentsResponse{Comments: comments}, nil
}

func (c *Client) DeleteArticleComment(ctx context.Context, slug string, publicID int) error {
	requesterID, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return shared.ErrUnauthorized
	}
	if publicID < 0 {
		return shared.NotFound("comment")
	}
	articleID, err := c.articles.ResolveArticleID(ctx, slug)
	if err != nil {
		return err
	}
	_, err = c.client.DeleteComment(ctx, &commentsv1.DeleteCommentRequest{
		ArticleId: articleID.String(), PublicId: uint64(publicID), RequesterId: requesterID.String(),
	})
	return grpcshared.DecodeError(err)
}

func (c *Client) hydrate(ctx context.Context, values []*commentsv1.Comment) ([]models.Comment, error) {
	if len(values) == 0 {
		return []models.Comment{}, nil
	}
	authorIDs := make([]uuid.UUID, 0, len(values))
	parsed := make([]uuid.UUID, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))
	for i, value := range values {
		if value == nil {
			return nil, errors.New("comments response contains a nil comment")
		}
		id, err := uuid.Parse(value.GetAuthorId())
		if err != nil {
			return nil, fmt.Errorf("parse comment author ID: %w", err)
		}
		parsed[i] = id
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			authorIDs = append(authorIDs, id)
		}
	}
	profiles, err := c.profiles.ProfilesByIDs(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	if viewerID, viewerErr := shared.UserIDFromContext(ctx); viewerErr == nil && c.subscriptions != nil {
		following, followErr := c.subscriptions.FollowingIDs(ctx, viewerID, authorIDs)
		if followErr != nil {
			return nil, followErr
		}
		for id := range following {
			profile := profiles[id]
			profile.Following = true
			profiles[id] = profile
		}
	}
	result := make([]models.Comment, len(values))
	for i, value := range values {
		profile, ok := profiles[parsed[i]]
		if !ok {
			return nil, fmt.Errorf("profile for comment author %s was not returned", parsed[i])
		}
		publicID, publicIDErr := safePublicID(value.GetPublicId())
		if publicIDErr != nil {
			return nil, publicIDErr
		}
		result[i] = models.Comment{
			ID: publicID, Body: value.GetBody(), Author: profile,
			CreatedAt: value.GetCreatedAt().AsTime(), UpdatedAt: value.GetUpdatedAt().AsTime(),
		}
	}
	return result, nil
}

func safePublicID(value uint64) (int, error) {
	if value > uint64(math.MaxInt) {
		return 0, fmt.Errorf("comment public ID %d overflows int", value)
	}
	return int(value), nil
}
