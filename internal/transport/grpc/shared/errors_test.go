package shared

import (
	"errors"
	"testing"

	serviceshared "conduit/internal/service/shared"

	"github.com/stretchr/testify/require"
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
		{name: "unauthorized", err: serviceshared.ErrUnauthorized, want: serviceshared.ErrUnauthorized, code: codes.Unauthenticated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			encoded := EncodeError(tt.err)
			require.Equal(t, tt.code, status.Code(encoded))
			decoded := DecodeError(encoded)
			require.ErrorIs(t, decoded, tt.want)

			var sourceAPI, decodedAPI *serviceshared.APIError
			if errors.As(tt.err, &sourceAPI) {
				require.ErrorAs(t, decoded, &decodedAPI)
				require.Equal(t, sourceAPI, decodedAPI)
			}
		})
	}
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
