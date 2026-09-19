package comments

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	commentsv1 "conduit/internal/gen/grpc/conduit/comments/v1"
	commonv1 "conduit/internal/gen/grpc/conduit/common/v1"
	"conduit/internal/service/comment"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRegister(t *testing.T) {
	server := grpc.NewServer()
	Register(server, &fakeService{})
	_, ok := server.GetServiceInfo()[commentsv1.CommentsService_ServiceDesc.ServiceName]
	require.True(t, ok)
}

type fakeService struct {
	create func(context.Context, uuid.UUID, uuid.UUID, string) (comment.Record, error)
	get    func(context.Context, uuid.UUID, int) (comment.Record, error)
	list   func(context.Context, uuid.UUID) ([]comment.Record, error)
	delete func(context.Context, uuid.UUID, int, uuid.UUID) error
}

func (f *fakeService) Create(ctx context.Context, articleID, authorID uuid.UUID, body string) (comment.Record, error) {
	return f.create(ctx, articleID, authorID, body)
}
func (f *fakeService) Get(ctx context.Context, articleID uuid.UUID, publicID int) (comment.Record, error) {
	return f.get(ctx, articleID, publicID)
}
func (f *fakeService) List(ctx context.Context, articleID uuid.UUID) ([]comment.Record, error) {
	return f.list(ctx, articleID)
}
func (f *fakeService) Delete(ctx context.Context, articleID uuid.UUID, publicID int, requesterID uuid.UUID) error {
	return f.delete(ctx, articleID, publicID, requesterID)
}

func TestServerCommentLifecycle(t *testing.T) {
	articleID, authorID := uuid.New(), uuid.New()
	now := time.Now().UTC()
	record := comment.Record{ID: uuid.New(), PublicID: 7, ArticleID: articleID, AuthorID: authorID, Body: "body", CreatedAt: now, UpdatedAt: now}
	service := &fakeService{
		create: func(_ context.Context, gotArticle, gotAuthor uuid.UUID, body string) (comment.Record, error) {
			require.Equal(t, articleID, gotArticle)
			require.Equal(t, authorID, gotAuthor)
			require.Equal(t, "body", body)
			return record, nil
		},
		get: func(_ context.Context, gotArticle uuid.UUID, publicID int) (comment.Record, error) {
			require.Equal(t, articleID, gotArticle)
			require.Equal(t, 7, publicID)
			return record, nil
		},
		list: func(context.Context, uuid.UUID) ([]comment.Record, error) {
			return []comment.Record{record, record, record}, nil
		},
		delete: func(_ context.Context, gotArticle uuid.UUID, publicID int, requester uuid.UUID) error {
			require.Equal(t, articleID, gotArticle)
			require.Equal(t, 7, publicID)
			require.Equal(t, authorID, requester)
			return nil
		},
	}
	server := NewServer(service)

	created, err := server.CreateComment(t.Context(), &commentsv1.CreateCommentRequest{ArticleId: articleID.String(), AuthorId: authorID.String(), Body: "body"})
	require.NoError(t, err)
	require.Equal(t, uint64(7), created.GetComment().GetPublicId())

	got, err := server.GetComment(t.Context(), &commentsv1.GetCommentRequest{ArticleId: articleID.String(), PublicId: 7})
	require.NoError(t, err)
	require.Equal(t, record.ID.String(), got.GetComment().GetId())

	listed, err := server.ListComments(t.Context(), &commentsv1.ListCommentsRequest{ArticleId: articleID.String()})
	require.NoError(t, err)
	require.Len(t, listed.GetComments(), 3)
	require.Equal(t, uint64(3), listed.GetCommentsCount())

	deleted, err := server.DeleteComment(t.Context(), &commentsv1.DeleteCommentRequest{ArticleId: articleID.String(), PublicId: 7, RequesterId: authorID.String()})
	require.NoError(t, err)
	require.True(t, deleted.GetDeleted())
}

func TestServerValidationAndErrors(t *testing.T) {
	server := NewServer(&fakeService{
		create: func(context.Context, uuid.UUID, uuid.UUID, string) (comment.Record, error) {
			return comment.Record{}, shared.Validation("body", "can't be blank")
		},
		get: func(context.Context, uuid.UUID, int) (comment.Record, error) {
			return comment.Record{}, shared.NotFound("comment")
		},
		list:   func(context.Context, uuid.UUID) ([]comment.Record, error) { return nil, errors.New("repository") },
		delete: func(context.Context, uuid.UUID, int, uuid.UUID) error { return shared.Forbidden("comment") },
	})
	articleID, authorID := uuid.New(), uuid.New()

	_, err := server.CreateComment(t.Context(), &commentsv1.CreateCommentRequest{ArticleId: "bad", AuthorId: authorID.String()})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.CreateComment(t.Context(), &commentsv1.CreateCommentRequest{ArticleId: articleID.String(), AuthorId: "bad"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.CreateComment(t.Context(), &commentsv1.CreateCommentRequest{ArticleId: articleID.String(), AuthorId: authorID.String()})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.GetComment(t.Context(), &commentsv1.GetCommentRequest{ArticleId: articleID.String(), PublicId: 7})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = server.ListComments(t.Context(), &commentsv1.ListCommentsRequest{ArticleId: articleID.String()})
	require.Equal(t, codes.Internal, status.Code(err))
	_, err = server.DeleteComment(t.Context(), &commentsv1.DeleteCommentRequest{ArticleId: articleID.String(), PublicId: 7, RequesterId: "bad"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.DeleteComment(t.Context(), &commentsv1.DeleteCommentRequest{ArticleId: articleID.String(), PublicId: 7, RequesterId: authorID.String()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestServerPaginates(t *testing.T) {
	articleID := uuid.New()
	server := NewServer(&fakeService{list: func(context.Context, uuid.UUID) ([]comment.Record, error) {
		return []comment.Record{{PublicID: 1}, {PublicID: 2}, {PublicID: 3}}, nil
	}})
	response, err := server.ListComments(t.Context(), &commentsv1.ListCommentsRequest{
		ArticleId: articleID.String(), Page: &commonv1.Page{Offset: 1, Limit: 1},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(2), response.GetComments()[0].GetPublicId())
	require.Equal(t, uint64(3), response.GetCommentsCount())
}

func TestServerRejectsInvalidKeysAndServiceErrors(t *testing.T) {
	t.Parallel()
	articleID, requesterID := uuid.New(), uuid.New()
	serviceErr := errors.New("service")
	server := NewServer(&fakeService{
		create: func(context.Context, uuid.UUID, uuid.UUID, string) (comment.Record, error) {
			return comment.Record{}, serviceErr
		},
		get:    func(context.Context, uuid.UUID, int) (comment.Record, error) { return comment.Record{}, serviceErr },
		list:   func(context.Context, uuid.UUID) ([]comment.Record, error) { return nil, serviceErr },
		delete: func(context.Context, uuid.UUID, int, uuid.UUID) error { return serviceErr },
	})

	_, err := server.CreateComment(t.Context(), &commentsv1.CreateCommentRequest{ArticleId: articleID.String(), AuthorId: requesterID.String(), Body: "body"})
	require.Equal(t, codes.Internal, status.Code(err))
	_, err = server.GetComment(t.Context(), &commentsv1.GetCommentRequest{ArticleId: "bad", PublicId: 1})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.GetComment(t.Context(), &commentsv1.GetCommentRequest{ArticleId: articleID.String(), PublicId: math.MaxUint64})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.GetComment(t.Context(), &commentsv1.GetCommentRequest{ArticleId: articleID.String(), PublicId: 1})
	require.Equal(t, codes.Internal, status.Code(err))
	_, err = server.ListComments(t.Context(), &commentsv1.ListCommentsRequest{ArticleId: "bad"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.DeleteComment(t.Context(), &commentsv1.DeleteCommentRequest{ArticleId: "bad", RequesterId: requesterID.String()})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.DeleteComment(t.Context(), &commentsv1.DeleteCommentRequest{ArticleId: articleID.String(), PublicId: math.MaxUint64, RequesterId: requesterID.String()})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = server.DeleteComment(t.Context(), &commentsv1.DeleteCommentRequest{ArticleId: articleID.String(), PublicId: 1, RequesterId: requesterID.String()})
	require.Equal(t, codes.Internal, status.Code(err))

	require.Zero(t, safeUint64(-1))
	require.Equal(t, uint64(7), safeUint64(7))
	start, end := pageBounds(3, 9, 1)
	require.Equal(t, 3, start)
	require.Equal(t, 3, end)
}
