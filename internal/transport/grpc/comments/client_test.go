package comments

import (
	"context"
	"errors"
	"testing"
	"time"

	commentsv1 "conduit/internal/gen/grpc/conduit/comments/v1"
	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeCommentsClient struct {
	create func(context.Context, *commentsv1.CreateCommentRequest) (*commentsv1.CreateCommentResponse, error)
	get    func(context.Context, *commentsv1.GetCommentRequest) (*commentsv1.GetCommentResponse, error)
	list   func(context.Context, *commentsv1.ListCommentsRequest) (*commentsv1.ListCommentsResponse, error)
	delete func(context.Context, *commentsv1.DeleteCommentRequest) (*commentsv1.DeleteCommentResponse, error)
}

func (f *fakeCommentsClient) CreateComment(ctx context.Context, request *commentsv1.CreateCommentRequest, _ ...grpc.CallOption) (*commentsv1.CreateCommentResponse, error) {
	return f.create(ctx, request)
}
func (f *fakeCommentsClient) GetComment(ctx context.Context, request *commentsv1.GetCommentRequest, _ ...grpc.CallOption) (*commentsv1.GetCommentResponse, error) {
	return f.get(ctx, request)
}
func (f *fakeCommentsClient) ListComments(ctx context.Context, request *commentsv1.ListCommentsRequest, _ ...grpc.CallOption) (*commentsv1.ListCommentsResponse, error) {
	return f.list(ctx, request)
}
func (f *fakeCommentsClient) DeleteComment(ctx context.Context, request *commentsv1.DeleteCommentRequest, _ ...grpc.CallOption) (*commentsv1.DeleteCommentResponse, error) {
	return f.delete(ctx, request)
}

type fakeResolver struct {
	id  uuid.UUID
	err error
}

func (f fakeResolver) ResolveArticleID(context.Context, string) (uuid.UUID, error) {
	return f.id, f.err
}

type fakeProfiles struct {
	profiles map[uuid.UUID]models.Profile
	err      error
	got      []uuid.UUID
}

type fakeSubscriptions struct {
	following map[uuid.UUID]struct{}
	got       []uuid.UUID
	err       error
}

func (f *fakeSubscriptions) FollowingIDs(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	f.got = ids
	return f.following, f.err
}

func (f *fakeProfiles) ProfilesByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]models.Profile, error) {
	f.got = ids
	return f.profiles, f.err
}

func TestClientHydratesFollowingInOneBatch(t *testing.T) {
	viewerID, authorID := uuid.New(), uuid.New()
	profiles := &fakeProfiles{profiles: map[uuid.UUID]models.Profile{authorID: {ID: authorID}}}
	subscriptions := &fakeSubscriptions{following: map[uuid.UUID]struct{}{authorID: {}}}
	client := NewApplicationClient(&fakeCommentsClient{}, fakeResolver{}, profiles, subscriptions)
	values, err := client.hydrate(shared.WithUserID(t.Context(), viewerID), []*commentsv1.Comment{{AuthorId: authorID.String(), CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()}})
	require.NoError(t, err)
	require.True(t, values[0].Author.Following)
	require.Equal(t, []uuid.UUID{authorID}, subscriptions.got)
}

func TestClientCreatesListsAndDeletes(t *testing.T) {
	articleID, authorID := uuid.New(), uuid.New()
	now := timestamppb.New(time.Now())
	remote := &commentsv1.Comment{PublicId: 7, AuthorId: authorID.String(), Body: "body", CreatedAt: now, UpdatedAt: now}
	grpcClient := &fakeCommentsClient{
		create: func(_ context.Context, request *commentsv1.CreateCommentRequest) (*commentsv1.CreateCommentResponse, error) {
			require.Equal(t, articleID.String(), request.GetArticleId())
			require.Equal(t, authorID.String(), request.GetAuthorId())
			return &commentsv1.CreateCommentResponse{Comment: remote}, nil
		},
		list: func(_ context.Context, request *commentsv1.ListCommentsRequest) (*commentsv1.ListCommentsResponse, error) {
			require.Equal(t, articleID.String(), request.GetArticleId())
			return &commentsv1.ListCommentsResponse{Comments: []*commentsv1.Comment{remote, remote}}, nil
		},
		delete: func(_ context.Context, request *commentsv1.DeleteCommentRequest) (*commentsv1.DeleteCommentResponse, error) {
			require.Equal(t, authorID.String(), request.GetRequesterId())
			return &commentsv1.DeleteCommentResponse{Deleted: true}, nil
		},
	}
	profiles := &fakeProfiles{profiles: map[uuid.UUID]models.Profile{authorID: {ID: authorID, Username: "author"}}}
	client := NewClient(grpcClient, fakeResolver{id: articleID}, profiles)
	ctx := shared.WithUserID(t.Context(), authorID)

	created, err := client.CreateArticleComment(ctx, "slug", models.NewCommentRequest{Comment: models.NewComment{Body: "body"}})
	require.NoError(t, err)
	require.Equal(t, "author", created.Comment.Author.Username)
	listed, err := client.GetArticleComments(ctx, "slug")
	require.NoError(t, err)
	require.Len(t, listed.Comments, 2)
	require.Equal(t, []uuid.UUID{authorID}, profiles.got)
	require.NoError(t, client.DeleteArticleComment(ctx, "slug", 7))
}

func TestClientMapsFailures(t *testing.T) {
	articleID, authorID := uuid.New(), uuid.New()
	remoteErr := status.Error(codes.NotFound, "missing")
	dependencyErr := errors.New("dependency")
	empty := &fakeCommentsClient{
		create: func(context.Context, *commentsv1.CreateCommentRequest) (*commentsv1.CreateCommentResponse, error) {
			return nil, remoteErr
		},
		list: func(context.Context, *commentsv1.ListCommentsRequest) (*commentsv1.ListCommentsResponse, error) {
			return nil, remoteErr
		},
		delete: func(context.Context, *commentsv1.DeleteCommentRequest) (*commentsv1.DeleteCommentResponse, error) {
			return nil, remoteErr
		},
	}
	client := NewClient(empty, fakeResolver{id: articleID}, &fakeProfiles{})
	_, err := client.CreateArticleComment(t.Context(), "slug", models.NewCommentRequest{Comment: models.NewComment{Body: "body"}})
	require.ErrorIs(t, err, shared.ErrUnauthorized)
	_, err = client.CreateArticleComment(shared.WithUserID(t.Context(), authorID), "slug", models.NewCommentRequest{})
	require.ErrorIs(t, err, shared.ErrValidation)
	_, err = client.GetArticleComments(t.Context(), "slug")
	require.Error(t, err)
	require.Error(t, client.DeleteArticleComment(shared.WithUserID(t.Context(), authorID), "slug", 7))
	require.ErrorIs(t, NewClient(empty, fakeResolver{err: dependencyErr}, &fakeProfiles{}).DeleteArticleComment(shared.WithUserID(t.Context(), authorID), "slug", 7), dependencyErr)
	require.ErrorIs(t, client.DeleteArticleComment(t.Context(), "slug", 7), shared.ErrUnauthorized)
}

func TestClientRejectsMalformedResponses(t *testing.T) {
	articleID, authorID := uuid.New(), uuid.New()
	profiles := &fakeProfiles{profiles: map[uuid.UUID]models.Profile{}}
	client := NewClient(&fakeCommentsClient{}, fakeResolver{id: articleID}, profiles)
	empty, err := client.hydrate(t.Context(), nil)
	require.NoError(t, err)
	require.Empty(t, empty)
	_, err = client.hydrate(t.Context(), []*commentsv1.Comment{nil})
	require.ErrorContains(t, err, "nil comment")
	_, err = client.hydrate(t.Context(), []*commentsv1.Comment{{AuthorId: "bad"}})
	require.ErrorContains(t, err, "author ID")
	_, err = client.hydrate(t.Context(), []*commentsv1.Comment{{AuthorId: authorID.String()}})
	require.ErrorContains(t, err, "was not returned")
	profiles.err = errors.New("profiles")
	_, err = client.hydrate(t.Context(), []*commentsv1.Comment{{AuthorId: authorID.String()}})
	require.ErrorIs(t, err, profiles.err)
}

func TestClientDependencyAndRemoteFailures(t *testing.T) {
	t.Parallel()
	articleID, authorID := uuid.New(), uuid.New()
	dependencyErr := errors.New("dependency")
	remoteErr := status.Error(codes.Unavailable, "unavailable")
	remote := &fakeCommentsClient{
		create: func(context.Context, *commentsv1.CreateCommentRequest) (*commentsv1.CreateCommentResponse, error) {
			return nil, remoteErr
		},
		list: func(context.Context, *commentsv1.ListCommentsRequest) (*commentsv1.ListCommentsResponse, error) {
			return nil, remoteErr
		},
		delete: func(context.Context, *commentsv1.DeleteCommentRequest) (*commentsv1.DeleteCommentResponse, error) {
			return nil, remoteErr
		},
	}
	ctx := shared.WithUserID(t.Context(), authorID)
	_, err := NewClient(remote, fakeResolver{err: dependencyErr}, &fakeProfiles{}).CreateArticleComment(ctx, "slug", models.NewCommentRequest{Comment: models.NewComment{Body: "body"}})
	require.ErrorIs(t, err, dependencyErr)
	_, err = NewClient(remote, fakeResolver{id: articleID}, &fakeProfiles{}).CreateArticleComment(ctx, "slug", models.NewCommentRequest{Comment: models.NewComment{Body: "body"}})
	require.Error(t, err)
	_, err = NewClient(remote, fakeResolver{err: dependencyErr}, &fakeProfiles{}).GetArticleComments(t.Context(), "slug")
	require.ErrorIs(t, err, dependencyErr)
	_, err = NewClient(remote, fakeResolver{id: articleID}, &fakeProfiles{}).GetArticleComments(t.Context(), "slug")
	require.Error(t, err)
	require.ErrorIs(t, NewClient(remote, fakeResolver{id: articleID}, &fakeProfiles{}).DeleteArticleComment(ctx, "slug", -1), shared.ErrNotFound)
}

func TestClientHydrationSubscriptionAndPublicIDFailures(t *testing.T) {
	t.Parallel()
	viewerID, authorID := uuid.New(), uuid.New()
	profile := models.Profile{ID: authorID}
	profiles := &fakeProfiles{profiles: map[uuid.UUID]models.Profile{authorID: profile}}
	comment := &commentsv1.Comment{AuthorId: authorID.String(), PublicId: ^uint64(0), CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()}
	dependencyErr := errors.New("subscriptions")
	client := NewApplicationClient(&fakeCommentsClient{}, fakeResolver{}, profiles, &fakeSubscriptions{err: dependencyErr})
	_, err := client.hydrate(shared.WithUserID(t.Context(), viewerID), []*commentsv1.Comment{comment})
	require.ErrorIs(t, err, dependencyErr)

	client = NewApplicationClient(&fakeCommentsClient{}, fakeResolver{}, profiles, nil)
	_, err = client.hydrate(t.Context(), []*commentsv1.Comment{comment})
	require.ErrorContains(t, err, "overflows int")
	value, err := safePublicID(7)
	require.NoError(t, err)
	require.Equal(t, 7, value)
}

func TestClientSuccessfulRPCWithInvalidPayload(t *testing.T) {
	t.Parallel()
	articleID, authorID := uuid.New(), uuid.New()
	remote := &fakeCommentsClient{
		create: func(context.Context, *commentsv1.CreateCommentRequest) (*commentsv1.CreateCommentResponse, error) {
			return &commentsv1.CreateCommentResponse{}, nil
		},
		list: func(context.Context, *commentsv1.ListCommentsRequest) (*commentsv1.ListCommentsResponse, error) {
			return &commentsv1.ListCommentsResponse{Comments: []*commentsv1.Comment{nil}}, nil
		},
	}
	client := NewClient(remote, fakeResolver{id: articleID}, &fakeProfiles{})
	ctx := shared.WithUserID(t.Context(), authorID)
	_, err := client.CreateArticleComment(ctx, "slug", models.NewCommentRequest{Comment: models.NewComment{Body: "body"}})
	require.ErrorContains(t, err, "nil comment")
	_, err = client.GetArticleComments(ctx, "slug")
	require.ErrorContains(t, err, "nil comment")
}
