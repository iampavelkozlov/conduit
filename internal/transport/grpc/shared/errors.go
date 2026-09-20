package shared

import (
	"errors"
	"net/http"

	serviceshared "conduit/internal/service/shared"

	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const errorDomain = "conduit.internal"

// EncodeError converts domain errors at the gRPC boundary while preserving the
// RealWorld error field and message for the HTTP gateway.
func EncodeError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}

	code := codes.Internal
	reason := "internal"
	switch {
	case errors.Is(err, serviceshared.ErrUnauthorized):
		code, reason = codes.Unauthenticated, "unauthorized"
	case errors.Is(err, serviceshared.ErrForbidden):
		code, reason = codes.PermissionDenied, "forbidden"
	case errors.Is(err, serviceshared.ErrNotFound):
		code, reason = codes.NotFound, "not_found"
	case errors.Is(err, serviceshared.ErrValidation):
		code, reason = codes.InvalidArgument, "validation"
	}

	message := "internal error"
	metadata := map[string]string{}
	if apiErr, ok := errors.AsType[*serviceshared.APIError](err); ok {
		switch apiErr.Status {
		case http.StatusUnauthorized:
			code, reason = codes.Unauthenticated, "unauthorized"
		case http.StatusForbidden:
			code, reason = codes.PermissionDenied, "forbidden"
		case http.StatusNotFound:
			code, reason = codes.NotFound, "not_found"
		case http.StatusConflict:
			code, reason = codes.AlreadyExists, "conflict"
		case http.StatusUnprocessableEntity:
			code, reason = codes.InvalidArgument, "validation"
		}
		message = apiErr.Message
		metadata["field"] = apiErr.Field
		metadata["message"] = apiErr.Message
	} else if code != codes.Internal {
		message = err.Error()
	}
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
		code, reason, message = codes.AlreadyExists, "conflict", "has already been taken"
		metadata["field"] = uniqueField(pgErr.ConstraintName)
		metadata["message"] = message
	}

	withDetails, detailErr := status.New(code, message).WithDetails(&errdetails.ErrorInfo{
		Reason: reason, Domain: errorDomain, Metadata: metadata,
	})
	if detailErr != nil {
		return status.Error(code, message)
	}
	return withDetails.Err()
}

// DecodeError converts a remote status back into the domain error vocabulary
// understood by the unchanged HTTP transport.
func DecodeError(err error) error {
	if err == nil {
		return nil
	}
	remote, ok := status.FromError(err)
	if !ok {
		return err
	}

	for _, detail := range remote.Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if !ok || info.Domain != errorDomain {
			continue
		}
		field, message := info.Metadata["field"], info.Metadata["message"]
		if field != "" && message != "" {
			switch remote.Code() {
			case codes.InvalidArgument:
				return &serviceshared.APIError{Status: http.StatusUnprocessableEntity, Field: field, Message: message}
			case codes.NotFound:
				return &serviceshared.APIError{Status: http.StatusNotFound, Field: field, Message: message}
			case codes.PermissionDenied:
				return &serviceshared.APIError{Status: http.StatusForbidden, Field: field, Message: message}
			case codes.AlreadyExists:
				return &serviceshared.APIError{Status: http.StatusConflict, Field: field, Message: message}
			case codes.Unauthenticated:
				return &serviceshared.APIError{Status: http.StatusUnauthorized, Field: field, Message: message}
			default:
			}
		}
	}

	switch remote.Code() {
	case codes.Unauthenticated:
		return serviceshared.ErrUnauthorized
	case codes.PermissionDenied:
		return serviceshared.ErrForbidden
	case codes.NotFound:
		return serviceshared.ErrNotFound
	case codes.InvalidArgument, codes.AlreadyExists:
		return serviceshared.ErrValidation
	default:
		return err
	}
}

func uniqueField(constraint string) string {
	switch constraint {
	case "users_username_key":
		return "username"
	case "users_email_key":
		return "email"
	default:
		return "body"
	}
}
