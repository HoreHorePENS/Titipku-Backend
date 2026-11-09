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
	// AccountType: "penitip" (default) atau "pemilik_toko".
	AccountType  string
	BusinessName string
	OwnerName    string
	Email        string
	WhatsApp     string
	Password     string
	// Store hanya diisi saat AccountType = "pemilik_toko".
	Store *RegisterStoreInput
}

// RegisterStoreInput adalah data toko yang dibuat saat pendaftaran pemilik toko.
type RegisterStoreInput struct {
	Name        string
	Category    string
	Description string
	Address     string
	WhatsApp    string
	Latitude    float64
	Longitude   float64
	PhotoURL    string
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
	accountType := strings.TrimSpace(input.AccountType)
	if accountType == "" {
		accountType = string(identity.AccountTypePenitip)
	}
	if !identity.ValidAccountType(accountType) {
		return nil, apperror.Validation(map[string]string{
			"account_type": "must be 'penitip' or 'pemilik_toko'",
		})
	}

	ownerName := strings.TrimSpace(input.OwnerName)
	email := domain.NormalizeEmail(input.Email)

	fields := make(map[string]string)
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

	var store *domain.Store
	if accountType == string(identity.AccountTypePemilikToko) {
		storeFields, validatedStore := validateRegisterStore(input, whatsApp)
		for key, message := range storeFields {
			fields[key] = message
		}
		store = validatedStore
	} else {
		businessName := strings.TrimSpace(input.BusinessName)
		if !domain.IsValidName(businessName) {
			fields["business_name"] = "must be between 2 and 120 characters"
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
	user := domain.User{
		ID:          uuid.New(),
		Email:       email,
		FullName:    ownerName,
		Role:        string(identity.RoleOwner),
		AccountType: accountType,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	var account domain.Account
	if accountType == string(identity.AccountTypePemilikToko) {
		store.ID = uuid.New()
		store.OwnerUserID = user.ID
		store.CreatedAt = now
		store.UpdatedAt = now
		account = domain.Account{User: user, Store: store}
	} else {
		businessID := uuid.New()
		account = domain.Account{
			User: user,
			Member: domain.Member{
				ID:         uuid.New(),
				BusinessID: businessID,
				UserID:     user.ID,
				Role:       string(identity.RoleOwner),
				IsActive:   true,
				CreatedAt:  now,
				UpdatedAt:  now,
			},
			Business: domain.Business{
				ID:        businessID,
				Name:      strings.TrimSpace(input.BusinessName),
				OwnerName: ownerName,
				WhatsApp:  whatsApp,
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
	}

	if err := a.store.CreateAccount(ctx, account, passwordHash); err != nil {
		return nil, mapStoreError(err, "ACCOUNT_CREATION_FAILED", "unable to create account")
	}

	return a.issueToken(account)
}

// validateRegisterStore memvalidasi data toko pemilik toko dan mengembalikan
// field yang tidak valid beserta store siap pakai (nil bila ada error).
func validateRegisterStore(input RegisterInput, fallbackWhatsApp *string) (map[string]string, *domain.Store) {
	fields := make(map[string]string)

	if input.Store == nil {
		return map[string]string{
			"store": "store details are required for pemilik_toko account",
		}, nil
	}

	name := strings.TrimSpace(input.Store.Name)
	if !domain.IsValidName(name) {
		fields["store_name"] = "must be between 2 and 120 characters"
	}
	category := strings.TrimSpace(input.Store.Category)
	if category == "" {
		category = "lainnya"
	}
	if !domain.IsValidStoreCategory(category) {
		fields["store_category"] = "must be one of " + strings.Join(domain.StoreCategories, ", ")
	}
	description := strings.TrimSpace(input.Store.Description)
	if !domain.IsValidDescription(description) {
		fields["store_description"] = "must be at most 1000 characters"
	}
	address := strings.TrimSpace(input.Store.Address)
	if !domain.IsValidAddress(address) {
		fields["store_address"] = "must be between 3 and 300 characters"
	}
	if !domain.IsValidLatitude(input.Store.Latitude) {
		fields["store_latitude"] = "must be between -90 and 90"
	}
	if !domain.IsValidLongitude(input.Store.Longitude) {
		fields["store_longitude"] = "must be between -180 and 180"
	}
	photoURL := strings.TrimSpace(input.Store.PhotoURL)
	if len(photoURL) > 500 {
		fields["store_photo_url"] = "must be at most 500 characters"
	}

	whatsApp := strings.TrimSpace(input.Store.WhatsApp)
	if whatsApp == "" && fallbackWhatsApp != nil {
		whatsApp = *fallbackWhatsApp
	}
	normalizedWhatsApp := ""
	if whatsApp != "" {
		normalized, valid := domain.NormalizeWhatsApp(whatsApp)
		if !valid {
			fields["store_whatsapp"] = "must be a valid Indonesian whatsapp number"
		} else {
			normalizedWhatsApp = normalized
		}
	} else {
		fields["store_whatsapp"] = "is required"
	}

	if len(fields) > 0 {
		return fields, nil
	}

	store := &domain.Store{
		Name:      name,
		Category:  category,
		Address:   address,
		WhatsApp:  normalizedWhatsApp,
		Latitude:  input.Store.Latitude,
		Longitude: input.Store.Longitude,
		IsActive:  true,
	}
	if description != "" {
		store.Description = &description
	}
	if photoURL != "" {
		store.PhotoURL = &photoURL
	}

	return fields, store
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

	accountType := identity.AccountType(account.User.AccountType)
	if !accountType.Valid() {
		accountType = identity.AccountTypePenitip
	}

	// Akun pemilik toko tidak punya business/member scope.
	businessID, memberID := account.Business.ID, account.Member.ID
	if accountType == identity.AccountTypePemilikToko {
		businessID, memberID = uuid.Nil, uuid.Nil
	}

	token, expiresAt, err := a.tokens.IssueScoped(
		account.User.ID,
		account.User.Email,
		account.User.FullName,
		role,
		accountType,
		businessID,
		memberID,
	)
	if err != nil {
		return nil, apperror.Internal("TOKEN_ISSUE_FAILED", "unable to issue access token").WithCause(err)
	}

	return &AuthResult{Account: account, Token: token, ExpiresAt: expiresAt}, nil
}
