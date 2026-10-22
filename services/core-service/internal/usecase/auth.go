package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

type RegisterInput struct {
	BusinessName string
	OwnerName    string
	Email        string
	WhatsApp     string
	Password     string
}

type LoginInput struct {
	Email    string
	Password string
}

type AuthResult struct {
	Account   domain.Account
	Token     string
	ExpiresAt time.Time
}

func (a *App) Register(ctx context.Context, input RegisterInput) (*AuthResult, error) {
	businessName := strings.TrimSpace(input.BusinessName)
	ownerName := strings.TrimSpace(input.OwnerName)
	email := domain.NormalizeEmail(input.Email)

	fields := make(map[string]string)
	if !domain.IsValidName(businessName) {
		fields["business_name"] = "must be between 2 and 120 characters"
	}
	if !domain.IsValidName(ownerName) {
		fields["owner_name"] = "must be between 2 and 120 characters"
	}
	if !domain.IsValidEmail(email) {
		fields["email"] = "must be a valid email address"
	}
	if !domain.IsValidPassword(input.Password) {
		fields["password"] = "must be between 8 and 72 characters"
	}

	var whatsApp *string
	if strings.TrimSpace(input.WhatsApp) != "" {
		normalized, valid := domain.NormalizeWhatsApp(input.WhatsApp)
		if !valid {
			fields["whatsapp"] = "must be a valid Indonesian whatsapp number"
		} else {
			whatsApp = &normalized
		}
	}

	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	passwordHash, err := a.hasher.Hash(input.Password)
	if err != nil {
		return nil, apperror.Internal("PASSWORD_HASH_FAILED", "unable to secure password").WithCause(err)
	}

	now := a.now()
	businessID := uuid.New()
	user := domain.User{
		ID:        uuid.New(),
		Email:     email,
		FullName:  ownerName,
		Role:      string(identity.RoleOwner),
		CreatedAt: now,
		UpdatedAt: now,
	}
	member := domain.Member{
		ID:         uuid.New(),
		BusinessID: businessID,
		UserID:     user.ID,
		Role:       string(identity.RoleOwner),
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	business := domain.Business{
		ID:        businessID,
		Name:      businessName,
		OwnerName: ownerName,
		WhatsApp:  whatsApp,
		CreatedAt: now,
		UpdatedAt: now,
	}
	account := domain.Account{User: user, Member: member, Business: business}

	if err := a.store.CreateAccount(ctx, account, passwordHash); err != nil {
		return nil, mapStoreError(err, "ACCOUNT_CREATION_FAILED", "unable to create account")
	}

	return a.issueToken(account)
}

func (a *App) Login(ctx context.Context, input LoginInput) (*AuthResult, error) {
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

	account, err := a.store.AccountByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.Unauthorized("INVALID_CREDENTIALS", "invalid email or password")
		}
		return nil, apperror.Internal("LOGIN_FAILED", "unable to authenticate user").WithCause(err)
	}

	if err := a.hasher.Compare(account.User.PasswordHash, input.Password); err != nil {
		return nil, apperror.Unauthorized("INVALID_CREDENTIALS", "invalid email or password")
	}

	return a.issueToken(*account)
}

func (a *App) Account(ctx context.Context, userID uuid.UUID) (*domain.Account, error) {
	account, err := a.store.AccountByUserID(ctx, userID)
	if err != nil {
		return nil, mapStoreError(err, "ACCOUNT_LOOKUP_FAILED", "unable to load account")
	}
	return account, nil
}

func (a *App) issueToken(account domain.Account) (*AuthResult, error) {
	role := identity.Role(account.User.Role)
	if !role.Valid() {
		role = identity.RoleOwner
	}

	token, expiresAt, err := a.tokens.IssueScoped(
		account.User.ID,
		account.User.Email,
		account.User.FullName,
		role,
		account.Business.ID,
		account.Member.ID,
	)
	if err != nil {
		return nil, apperror.Internal("TOKEN_ISSUE_FAILED", "unable to issue access token").WithCause(err)
	}

	return &AuthResult{Account: account, Token: token, ExpiresAt: expiresAt}, nil
}
