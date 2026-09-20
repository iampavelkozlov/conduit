package profile

import (
	"context"

	commonv1 "conduit/internal/gen/grpc/conduit/common/v1"
	profilev1 "conduit/internal/gen/grpc/conduit/profile/v1"
	profiledomain "conduit/internal/service/profile"
	"conduit/internal/service/shared"
	grpcshared "conduit/internal/transport/grpc/shared"

	"github.com/google/uuid"
)

type service interface {
	Create(context.Context, uuid.UUID, string) (profiledomain.Profile, bool, error)
	Get(context.Context, uuid.UUID) (profiledomain.Profile, error)
	GetByUsername(context.Context, string) (profiledomain.Profile, error)
	BatchGet(context.Context, []uuid.UUID) ([]profiledomain.Profile, error)
	Update(context.Context, uuid.UUID, profiledomain.Update) (profiledomain.Profile, error)
}

type Server struct {
	profilev1.UnimplementedProfileServiceServer
	service service
}

func NewServer(service service) *Server {
	return &Server{service: service}
}

func (s *Server) CreateProfile(ctx context.Context, request *profilev1.CreateProfileRequest) (*profilev1.CreateProfileResponse, error) {
	id, err := parseID(request.GetUserId(), "user_id")
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	profile, created, err := s.service.Create(ctx, id, request.GetUsername())
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &profilev1.CreateProfileResponse{Profile: toProto(profile), Created: created}, nil
}

func (s *Server) GetProfile(ctx context.Context, request *profilev1.GetProfileRequest) (*profilev1.GetProfileResponse, error) {
	id, err := parseID(request.GetUserId(), "user_id")
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	profile, err := s.service.Get(ctx, id)
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &profilev1.GetProfileResponse{Profile: toProto(profile)}, nil
}

func (s *Server) GetProfileByUsername(ctx context.Context, request *profilev1.GetProfileByUsernameRequest) (*profilev1.GetProfileByUsernameResponse, error) {
	profile, err := s.service.GetByUsername(ctx, request.GetUsername())
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &profilev1.GetProfileByUsernameResponse{Profile: toProto(profile)}, nil
}

func (s *Server) BatchGetProfiles(ctx context.Context, request *profilev1.BatchGetProfilesRequest) (*profilev1.BatchGetProfilesResponse, error) {
	ids := make([]uuid.UUID, len(request.GetUserIds()))
	for i, value := range request.GetUserIds() {
		id, err := parseID(value, "user_ids")
		if err != nil {
			return nil, grpcshared.EncodeError(err)
		}
		ids[i] = id
	}
	profiles, err := s.service.BatchGet(ctx, ids)
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	result := make([]*profilev1.Profile, len(profiles))
	for i := range profiles {
		result[i] = toProto(profiles[i])
	}
	return &profilev1.BatchGetProfilesResponse{Profiles: result}, nil
}

func (s *Server) UpdateProfile(ctx context.Context, request *profilev1.UpdateProfileRequest) (*profilev1.UpdateProfileResponse, error) {
	id, err := parseID(request.GetUserId(), "user_id")
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	update := profiledomain.Update{Username: request.Username}
	update.Bio, update.BioSet = nullableValue(request.GetBio())
	update.Image, update.ImageSet = nullableValue(request.GetImage())
	profile, err := s.service.Update(ctx, id, update)
	if err != nil {
		return nil, grpcshared.EncodeError(err)
	}
	return &profilev1.UpdateProfileResponse{Profile: toProto(profile)}, nil
}

func parseID(value, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, shared.Validation(field, "must be a valid UUID")
	}
	return id, nil
}

func nullableValue(value *commonv1.NullableString) (*string, bool) {
	if value == nil {
		return nil, false
	}
	if _, ok := value.GetKind().(*commonv1.NullableString_Null); ok {
		return nil, true
	}
	result := value.GetValue()
	return &result, true
}

func toProto(profile profiledomain.Profile) *profilev1.Profile {
	return &profilev1.Profile{
		UserId: profile.UserID.String(), Username: profile.Username, Bio: profile.Bio, Image: profile.Image,
	}
}

var _ profilev1.ProfileServiceServer = (*Server)(nil)
