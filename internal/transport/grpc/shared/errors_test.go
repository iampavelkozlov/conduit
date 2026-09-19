package shared

import (
	"errors"
	"testing"

	serviceshared "conduit/internal/service/shared"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrorRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want error
		code codes.Code
	}{
		{name: "validation detail", err: serviceshared.Validation("username", "can't be blank"), want: serviceshared.ErrValidation, code: codes.InvalidArgument},
		{name: "not found detail", err: serviceshared.NotFound("article"), want: serviceshared.ErrNotFound, code: codes.NotFound},
		{name: "forbidden detail", err: serviceshared.Forbidden("comment"), want: serviceshared.ErrForbidden, code: codes.PermissionDenied},
		{name: "unauthorized detail", err: &serviceshared.APIError{Status: 401, Field: "credentials", Message: "invalid"}, code: codes.Unauthenticated},
		{name: "unauthorized", err: serviceshared.ErrUnauthorized, want: serviceshared.ErrUnauthorized, code: codes.Unauthenticated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			encoded := EncodeError(tt.err)
			require.Equal(t, tt.code, status.Code(encoded))
			decoded := DecodeError(encoded)
			if tt.want != nil {
				require.ErrorIs(t, decoded, tt.want)
			}

			var sourceAPI, decodedAPI *serviceshared.APIError
			if errors.As(tt.err, &sourceAPI) {
				require.ErrorAs(t, decoded, &decodedAPI)
				require.Equal(t, sourceAPI, decodedAPI)
			}
		})
	}
}

func TestUniqueViolationRoundTrip(t *testing.T) {
	t.Parallel()
	encoded := EncodeError(&pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"})
	require.Equal(t, codes.AlreadyExists, status.Code(encoded))

	decoded := DecodeError(encoded)
	var apiErr *serviceshared.APIError
	require.ErrorAs(t, decoded, &apiErr)
	require.Equal(t, 409, apiErr.Status)
	require.Equal(t, "email", apiErr.Field)
	require.Equal(t, "has already been taken", apiErr.Message)
}

func TestEncodeErrorPreservesExistingStatus(t *testing.T) {
	t.Parallel()
	err := status.Error(codes.Unavailable, "offline")
	require.Same(t, err, EncodeError(err))
}

func TestDecodeErrorPreservesUnknownStatus(t *testing.T) {
	t.Parallel()
	err := status.Error(codes.Unavailable, "offline")
	require.Same(t, err, DecodeError(err))
}

func TestErrorBoundaryEdgeCases(t *testing.T) {
	t.Parallel()
	require.NoError(t, EncodeError(nil))
	require.NoError(t, DecodeError(nil))

	plain := errors.New("database offline")
	encoded := EncodeError(plain)
	require.Equal(t, codes.Internal, status.Code(encoded))
	require.Equal(t, "internal error", status.Convert(encoded).Message())
	require.Same(t, plain, DecodeError(plain))

	for _, code := range []codes.Code{codes.InvalidArgument, codes.AlreadyExists} {
		require.ErrorIs(t, DecodeError(status.Error(code, "conflict")), serviceshared.ErrValidation)
	}
}

func TestDecodeErrorIgnoresUnrecognizedDetails(t *testing.T) {
	t.Parallel()
	wrongDomain, err := status.New(codes.NotFound, "missing").WithDetails(&errdetails.ErrorInfo{
		Domain: "another.domain", Metadata: map[string]string{"field": "article", "message": "not found"},
	})
	require.NoError(t, err)
	require.ErrorIs(t, DecodeError(wrongDomain.Err()), serviceshared.ErrNotFound)

	wrongType, err := status.New(codes.PermissionDenied, "denied").WithDetails(&errdetails.DebugInfo{Detail: "debug"})
	require.NoError(t, err)
	require.ErrorIs(t, DecodeError(wrongType.Err()), serviceshared.ErrForbidden)

	missingMetadata, err := status.New(codes.InvalidArgument, "invalid").WithDetails(&errdetails.ErrorInfo{Domain: errorDomain})
	require.NoError(t, err)
	require.ErrorIs(t, DecodeError(missingMetadata.Err()), serviceshared.ErrValidation)

	unsupportedDetailedCode, err := status.New(codes.Unauthenticated, "invalid").WithDetails(&errdetails.ErrorInfo{
		Domain: errorDomain, Metadata: map[string]string{"field": "token", "message": "invalid"},
	})
	require.NoError(t, err)
	var unauthorized *serviceshared.APIError
	require.ErrorAs(t, DecodeError(unsupportedDetailedCode.Err()), &unauthorized)
	require.Equal(t, 401, unauthorized.Status)
}
