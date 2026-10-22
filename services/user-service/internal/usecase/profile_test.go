package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/services/user-service/internal/domain"
)

func TestUpsertCreatesProfile(t *testing.T) {
	now := time.Date(2026, time.February, 2, 8, 0, 0, 0, time.UTC)
	profiles := newFakeProfileRepository()
	service := NewService(profiles, func() time.Time { return now })
	userID := uuid.New()

	profile, err := service.Upsert(context.Background(), userID, UpsertInput{
		FullName:  "Budi Santoso",
		Phone:     "+62 812-3456-7890",
		AvatarURL: "https://example.com/avatar.png",
		Bio:       "Courier and part-time student",
	})
	if err != nil {
		t.Fatalf("upsert returned error: %v", err)
	}

	if profile.UserID != userID {
		t.Fatalf("unexpected user id: %s", profile.UserID)
	}
	if !profile.CreatedAt.Equal(now) {
		t.Fatalf("unexpected created at: %s", profile.CreatedAt)
	}
	if profiles.saved[userID].FullName != "Budi Santoso" {
		t.Fatal("profile was not saved")
	}
}

func TestUpsertRejectsInvalidProfile(t *testing.T) {
	service := NewService(newFakeProfileRepository(), time.Now)

	_, err := service.Upsert(context.Background(), uuid.New(), UpsertInput{
		FullName:  "B",
		Phone:     "12",
		AvatarURL: "ftp://example.com/avatar.png",
		Bio:       string(make([]byte, domain.MaxBioLength+1)),
	})

	if err == nil {
		t.Fatal("expected an error")
	}

	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected apperror, got %T", err)
	}
	if appErr.Kind != apperror.KindInvalid {
		t.Fatalf("expected invalid kind, got %v", appErr.Kind)
	}
	for _, field := range []string{"full_name", "phone", "avatar_url", "bio"} {
		if appErr.Fields[field] == "" {
			t.Fatalf("expected validation message for %s", field)
		}
	}
}

func TestGetReturnsNotFound(t *testing.T) {
	service := NewService(newFakeProfileRepository(), time.Now)

	_, err := service.Get(context.Background(), uuid.New())

	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected apperror, got %T", err)
	}
	if appErr.Kind != apperror.KindNotFound {
		t.Fatalf("expected not found kind, got %v", appErr.Kind)
	}
}

type fakeProfileRepository struct {
	saved map[uuid.UUID]domain.Profile
}

func newFakeProfileRepository() *fakeProfileRepository {
	return &fakeProfileRepository{saved: make(map[uuid.UUID]domain.Profile)}
}

func (r *fakeProfileRepository) ByUserID(_ context.Context, userID uuid.UUID) (*domain.Profile, error) {
	profile, exists := r.saved[userID]
	if !exists {
		return nil, domain.ErrProfileNotFound
	}
	return &profile, nil
}

func (r *fakeProfileRepository) Save(_ context.Context, profile *domain.Profile) error {
	r.saved[profile.UserID] = *profile
	return nil
}
