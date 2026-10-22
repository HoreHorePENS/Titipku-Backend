package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/identity"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrEmailTaken   = errors.New("email is already registered")
	ErrInvalidUser  = errors.New("invalid user")
)

const (
	MinFullNameLength = 2
	MaxFullNameLength = 100
	MinPasswordLength = 8
	MaxPasswordLength = identity.MaxPasswordLength
)

var emailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type User struct {
	ID           uuid.UUID
	Email        string
	FullName     string
	PasswordHash string
	Role         identity.Role
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewUser(
	id uuid.UUID,
	email string,
	fullName string,
	passwordHash string,
	role identity.Role,
	now time.Time,
) (User, error) {
	email = NormalizeEmail(email)
	fullName = strings.TrimSpace(fullName)

	if !IsValidEmail(email) {
		return User{}, ErrInvalidUser
	}
	if !IsValidFullName(fullName) {
		return User{}, ErrInvalidUser
	}
	if passwordHash == "" {
		return User{}, ErrInvalidUser
	}
	if !role.Valid() {
		return User{}, ErrInvalidUser
	}

	return User{
		ID:           id,
		Email:        email,
		FullName:     fullName,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func IsValidEmail(email string) bool {
	return len(email) <= 254 && emailPattern.MatchString(email)
}

func IsValidFullName(fullName string) bool {
	length := utf8.RuneCountInString(strings.TrimSpace(fullName))
	return length >= MinFullNameLength && length <= MaxFullNameLength
}

func IsValidPassword(password string) bool {
	return len(password) >= MinPasswordLength && len(password) <= MaxPasswordLength
}
