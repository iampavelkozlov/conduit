package comments

import (
	"context"
	"math"

	commentsv1 "conduit/internal/gen/grpc/conduit/comments/v1"
	"conduit/internal/service/comment"
	grpcshared "conduit/internal/transport/grpc/shared"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

//go:generate go run go.uber.org/mock/mockgen -source=server.go -destination=mock_service_test.go -package=comments
type service interface {
	Create(context.Context, uuid.UUID, uuid.UUID, string) (comment.Record, error)
	Get(context.Context, uuid.UUID, int) (comment.Record, error)
	List(context.Context, uuid.UUID) ([]comment.Record, error)
	Delete(context.Context, uuid.UUID, int, uuid.UUID) error
}

type Server struct {
	commentsv1.UnimplementedCommentsServiceServer
	service service
}

func NewServer(service service) *Server {
	return &Server{service: service}
}

func Register(registrar grpc.ServiceRegistrar, service service) {
	var handler commentsv1.CommentsServiceServer = NewServer(service)
	commentsv1.RegisterCommentsServiceServer(registrar, handler)
}

func (s *Server) CreateComment(ctx context.Context, request *commentsv1.CreateCommentRequest) (*commentsv1.CreateCommentResponse, error) {
	articleID, err := parseUUID("article_id", request.GetArticleId())
	if err != nil {
		return nil, err
	}
	authorID, err := parseUUID("author_id", request.GetAuthorId())
	if err != nil {
		return nil, err
	}
	record, err := s.service.Create(ctx, articleID, authorID, request.GetBody())
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &commentsv1.CreateCommentResponse{Comment: encodeComment(&record)}, nil
}

func (s *Server) GetComment(ctx context.Context, request *commentsv1.GetCommentRequest) (*commentsv1.GetCommentResponse, error) {
	articleID, publicID, err := parseCommentKey(request.GetArticleId(), request.GetPublicId())
	if err != nil {
		return nil, err
	}
	record, err := s.service.Get(ctx, articleID, publicID)
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &commentsv1.GetCommentResponse{Comment: encodeComment(&record)}, nil
}

func (s *Server) ListComments(ctx context.Context, request *commentsv1.ListCommentsRequest) (*commentsv1.ListCommentsResponse, error) {
	articleID, err := parseUUID("article_id", request.GetArticleId())
	if err != nil {
		return nil, err
	}
	records, err := s.service.List(ctx, articleID)
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	total := len(records)
	start, end := pageBounds(total, request.GetPage().GetOffset(), request.GetPage().GetLimit())
	encoded := make([]*commentsv1.Comment, end-start)
	for i := start; i < end; i++ {
		encoded[i-start] = encodeComment(&records[i])
	}
	return &commentsv1.ListCommentsResponse{Comments: encoded, CommentsCount: uint64(total)}, nil
}

func (s *Server) DeleteComment(ctx context.Context, request *commentsv1.DeleteCommentRequest) (*commentsv1.DeleteCommentResponse, error) {
	articleID, publicID, err := parseCommentKey(request.GetArticleId(), request.GetPublicId())
	if err != nil {
		return nil, err
	}
	requesterID, err := parseUUID("requester_id", request.GetRequesterId())
	if err != nil {
		return nil, err
	}
	if err := s.service.Delete(ctx, articleID, publicID, requesterID); err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &commentsv1.DeleteCommentResponse{Deleted: true}, nil
}

func parseCommentKey(article string, public uint64) (uuid.UUID, int, error) {
	articleID, err := parseUUID("article_id", article)
	if err != nil {
		return uuid.Nil, 0, err
	}
	if public > math.MaxInt {
		return uuid.Nil, 0, status.Error(codes.InvalidArgument, "public_id is out of range")
	}
	return articleID, int(public), nil
}

func parseUUID(field, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s must be a UUID", field)
	}
	return id, nil
}

func encodeComment(record *comment.Record) *commentsv1.Comment {
	return &commentsv1.Comment{
		Id: record.ID.String(), PublicId: safeUint64(record.PublicID), ArticleId: record.ArticleID.String(),
		AuthorId: record.AuthorID.String(), Body: record.Body,
		CreatedAt: timestamppb.New(record.CreatedAt), UpdatedAt: timestamppb.New(record.UpdatedAt),
	}
}

func safeUint64(value int) uint64 {
	if value < 0 {
		return 0
	}
	return uint64(value)
}

func pageBounds(length int, offset, limit uint32) (int, int) {
	start := min(int(offset), length)
	if limit == 0 {
		return start, length
	}
	return start, min(start+int(limit), length)
}
