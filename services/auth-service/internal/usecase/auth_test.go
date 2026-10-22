package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/services/auth-service/internal/domain"
)

func TestRegisterCreatesCustomerAndIssuesToken(t *testing.T) {
	now := time.Date(2026, time.January, 10, 9, 0, 0, 0, time.UTC)
	users := newFakeUserRepository()
	auth := NewAuth(users, fakeHasher{}, fixedTokens{now: now}, func() time.Time { return now })

	result, err := auth.Register(context.Background(), RegisterInput{
		Email:    "  Customer@Example.com ",
		FullName: "Siti Rahma",
		Password: "secure-password",
	})
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}

	if result.User.Email != "customer@example.com" {
		t.Fatalf("email was not normalized: %q", result.User.Email)
	}
	if result.User.Role != identity.RoleCustomer {
		t.Fatalf("unexpected role: %q", result.User.Role)
	}
	if result.User.PasswordHash == "secure-password" {
		t.Fatal("password was stored in plain text")
	}
	if result.AccessToken == "" {
		t.Fatal("expected an access token")
	}
	if !result.ExpiresAt.After(now) {
		t.Fatal("expected token expiry in the future")
	}
}

func TestRegisterRejectsInvalidInput(t *testing.T) {
	auth := NewAuth(newFakeUserRepository(), fakeHasher{}, fixedTokens{}, time.Now)

	_, err := auth.Register(context.Background(), RegisterInput{
		Email:    "not-an-email",
		FullName: "A",
		Password: "short",
	})

	appErr := requireAppError(t, err)
	if appErr.Kind != apperror.KindInvalid {
		t.Fatalf("expected invalid kind, got %v", appErr.Kind)
	}
	for _, field := range []string{"email", "full_name", "password"} {
		if appErr.Fields[field] == "" {
			t.Fatalf("expected validation message for %s", field)
		}
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	now := time.Now()
	users := newFakeUserRepository()
	user, err := domain.NewUser(uuid.New(), "customer@example.com", "Siti Rahma", "hash", identity.RoleCustomer, now)
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	if err := users.Create(context.Background(), &user); err != nil {
		t.Fatalf("seed repository: %v", err)
	}

	auth := NewAuth(users, fakeHasher{}, fixedTokens{}, time.Now)
	_, err = auth.Register(context.Background(), RegisterInput{
		Email:    "CUSTOMER@example.com",
		FullName: "Siti Rahma",
		Password: "secure-password",
	})

	appErr := requireAppError(t, err)
	if appErr.Kind != apperror.KindConflict || appErr.Code != "EMAIL_TAKEN" {
		t.Fatalf("unexpected error: %#v", appErr)
	}
}

func TestLoginReturnsInvalidCredentialsForWrongPassword(t *testing.T) {
	now := time.Now()
	users := newFakeUserRepository()
	auth := NewAuth(users, fakeHasher{}, fixedTokens{now: now}, func() time.Time { return now })

	if _, err := auth.Register(context.Background(), RegisterInput{
		Email:    "customer@example.com",
		FullName: "Siti Rahma",
		Password: "secure-password",
	}); err != nil {
		t.Fatalf("register fixture: %v", err)
	}

	_, err := auth.Login(context.Background(), LoginInput{
		Email:    "customer@example.com",
		Password: "wrong-password",
	})

	appErr := requireAppError(t, err)
	if appErr.Kind != apperror.KindUnauthorized || appErr.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("unexpected error: %#v", appErr)
	}
}

func TestLoginReturnsTokenForValidCredentials(t *testing.T) {
	now := time.Now()
	auth := NewAuth(newFakeUserRepository(), fakeHasher{}, fixedTokens{now: now}, func() time.Time { return now })

	if _, err := auth.Register(context.Background(), RegisterInput{
		Email:    "customer@example.com",
		FullName: "Siti Rahma",
		Password: "secure-password",
	}); err != nil {
		t.Fatalf("register fixture: %v", err)
	}

	result, err := auth.Login(context.Background(), LoginInput{
		Email:    "customer@example.com",
		Password: "secure-password",
	})
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}
	if result.AccessToken == "" {
		t.Fatal("expected an access token")
	}
}

func requireAppError(t *testing.T, err error) *apperror.Error {
	t.Helper()

	if err == nil {
		t.Fatal("expected an error")
	}

	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected apperror, got %T", err)
	}
	return appErr
}

type fakeUserRepository struct {
	byEmail map[string]domain.User
	byID    map[uuid.UUID]domain.User
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		byEmail: make(map[string]domain.User),
		byID:    make(map[uuid.UUID]domain.User),
	}
}

func (r *fakeUserRepository) Create(_ context.Context, user *domain.User) error {
	if _, exists := r.byEmail[user.Email]; exists {
		return domain.ErrEmailTaken
	}
	r.byEmail[user.Email] = *user
	r.byID[user.ID] = *user
	return nil
}

func (r *fakeUserRepository) ByEmail(_ context.Context, email string) (*domain.User, error) {
	user, exists := r.byEmail[email]
	if !exists {
		return nil, domain.ErrUserNotFound
	}
	return &user, nil
}

func (r *fakeUserRepository) ByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	user, exists := r.byID[id]
	if !exists {
		return nil, domain.ErrUserNotFound
	}
	return &user, nil
}

type fakeHasher struct{}

func (fakeHasher) Hash(plainPassword string) (string, error) {
	return "hashed:" + plainPassword, nil
}

func (fakeHasher) Compare(hash, plainPassword string) error {
	if hash != "hashed:"+plainPassword {
		return errors.New("password mismatch")
	}
	return nil
}

type fixedTokens struct {
	now time.Time
}

func (t fixedTokens) Issue(userID uuid.UUID, _, _ string, _ identity.Role) (string, time.Time, error) {
	if t.now.IsZero() {
		t.now = time.Now()
	}
	return "token-" + userID.String(), t.now.Add(time.Hour), nil
}
