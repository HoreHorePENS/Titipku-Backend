package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/services/auth-service/internal/domain"
)

type RegisterInput struct {
	Email    string
	FullName string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type Result struct {
	User        domain.User
	AccessToken string
	ExpiresAt   time.Time
}

type Auth struct {
	users  UserRepository
	hasher PasswordHasher
	tokens TokenIssuer
	now    func() time.Time
}

func NewAuth(users UserRepository, hasher PasswordHasher, tokens TokenIssuer, now func() time.Time) *Auth {
	if now == nil {
		now = time.Now
	}

	return &Auth{
		users:  users,
		hasher: hasher,
		tokens: tokens,
		now:    now,
	}
}

func (a *Auth) Register(ctx context.Context, input RegisterInput) (*Result, error) {
	email := domain.NormalizeEmail(input.Email)
	fullName := strings.TrimSpace(input.FullName)

	fields := make(map[string]string)
	if !domain.IsValidEmail(email) {
		fields["email"] = "must be a valid email address"
	}
	if !domain.IsValidFullName(fullName) {
		fields["full_name"] = "must be between 2 and 100 characters"
	}
	if !domain.IsValidPassword(input.Password) {
		fields["password"] = "must be between 8 and 72 characters"
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	passwordHash, err := a.hasher.Hash(input.Password)
	if err != nil {
		return nil, apperror.Internal("PASSWORD_HASH_FAILED", "unable to secure password").WithCause(err)
	}

	user, err := domain.NewUser(
		uuid.New(),
		email,
		fullName,
		passwordHash,
		identity.RoleCustomer,
		a.now(),
	)
	if err != nil {
		return nil, apperror.Internal("USER_CREATION_FAILED", "unable to create user").WithCause(err)
	}

	if err := a.users.Create(ctx, &user); err != nil {
		if errors.Is(err, domain.ErrEmailTaken) {
			return nil, apperror.Conflict("EMAIL_TAKEN", "email is already registered")
		}
		return nil, apperror.Internal("USER_CREATION_FAILED", "unable to create user").WithCause(err)
	}

	return a.issueToken(user)
}

func (a *Auth) Login(ctx context.Context, input LoginInput) (*Result, error) {
	email := domain.NormalizeEmail(input.Email)

	fields := make(map[string]string)
	if !domain.IsValidEmail(email) {
		fields["email"] = "must be a valid email address"
	}
	if input.Password == "" {
		fields["password"] = "is required"
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	user, err := a.users.ByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, apperror.Unauthorized("INVALID_CREDENTIALS", "invalid email or password")
		}
		return nil, apperror.Internal("LOGIN_FAILED", "unable to authenticate user").WithCause(err)
	}

	if err := a.hasher.Compare(user.PasswordHash, input.Password); err != nil {
		return nil, apperror.Unauthorized("INVALID_CREDENTIALS", "invalid email or password")
	}

	return a.issueToken(*user)
}

func (a *Auth) Profile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := a.users.ByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, apperror.NotFound("USER_NOT_FOUND", "user not found")
		}
		return nil, apperror.Internal("PROFILE_LOOKUP_FAILED", "unable to load user").WithCause(err)
	}
	return user, nil
}

func (a *Auth) issueToken(user domain.User) (*Result, error) {
	accessToken, expiresAt, err := a.tokens.Issue(user.ID, user.Email, user.FullName, user.Role)
	if err != nil {
		return nil, apperror.Internal("TOKEN_ISSUE_FAILED", "unable to issue access token").WithCause(err)
	}

	return &Result{
		User:        user,
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
	}, nil
}
