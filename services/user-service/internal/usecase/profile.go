package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/services/user-service/internal/domain"
)

type UpsertInput struct {
	FullName  string
	Phone     string
	AvatarURL string
	Bio       string
}

type Service struct {
	profiles ProfileRepository
	now      func() time.Time
}

func NewService(profiles ProfileRepository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}

	return &Service{
		profiles: profiles,
		now:      now,
	}
}

func (s *Service) Get(ctx context.Context, userID uuid.UUID) (*domain.Profile, error) {
	profile, err := s.profiles.ByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrProfileNotFound) {
			return nil, apperror.NotFound("PROFILE_NOT_FOUND", "profile not found")
		}
		return nil, apperror.Internal("PROFILE_LOOKUP_FAILED", "unable to load profile").WithCause(err)
	}
	return profile, nil
}

func (s *Service) Upsert(ctx context.Context, userID uuid.UUID, input UpsertInput) (*domain.Profile, error) {
	fullName := strings.TrimSpace(input.FullName)
	phone := strings.TrimSpace(input.Phone)
	avatarURL := strings.TrimSpace(input.AvatarURL)
	bio := strings.TrimSpace(input.Bio)

	fields := make(map[string]string)
	if !domain.IsValidFullName(fullName) {
		fields["full_name"] = "must be between 2 and 100 characters"
	}
	if !domain.IsValidPhone(phone) {
		fields["phone"] = "must be a valid phone number between 8 and 20 characters"
	}
	if !domain.IsValidAvatarURL(avatarURL) {
		fields["avatar_url"] = "must be a valid http or https URL"
	}
	if !domain.IsValidBio(bio) {
		fields["bio"] = "must not exceed 500 characters"
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	profile, err := domain.NewProfile(userID, fullName, phone, avatarURL, bio, s.now())
	if err != nil {
		return nil, apperror.Internal("PROFILE_CREATION_FAILED", "unable to create profile").WithCause(err)
	}

	if err := s.profiles.Save(ctx, &profile); err != nil {
		return nil, apperror.Internal("PROFILE_SAVE_FAILED", "unable to save profile").WithCause(err)
	}

	return &profile, nil
}
