package domain

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrProfileNotFound = errors.New("profile not found")
	ErrInvalidProfile  = errors.New("invalid profile")
)

const (
	MinFullNameLength  = 2
	MaxFullNameLength  = 100
	MinPhoneLength     = 8
	MaxPhoneLength     = 20
	MaxAvatarURLLength = 500
	MaxBioLength       = 500
)

var phonePattern = regexp.MustCompile(`^\+?[0-9][0-9\s\-()]+$`)

type Profile struct {
	UserID    uuid.UUID
	FullName  string
	Phone     string
	AvatarURL string
	Bio       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewProfile(
	userID uuid.UUID,
	fullName string,
	phone string,
	avatarURL string,
	bio string,
	now time.Time,
) (Profile, error) {
	fullName = strings.TrimSpace(fullName)
	phone = strings.TrimSpace(phone)
	avatarURL = strings.TrimSpace(avatarURL)
	bio = strings.TrimSpace(bio)

	if !IsValidFullName(fullName) || !IsValidPhone(phone) || !IsValidAvatarURL(avatarURL) || !IsValidBio(bio) {
		return Profile{}, ErrInvalidProfile
	}

	return Profile{
		UserID:    userID,
		FullName:  fullName,
		Phone:     phone,
		AvatarURL: avatarURL,
		Bio:       bio,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func IsValidFullName(fullName string) bool {
	length := utf8.RuneCountInString(strings.TrimSpace(fullName))
	return length >= MinFullNameLength && length <= MaxFullNameLength
}

func IsValidPhone(phone string) bool {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return true
	}

	return len(phone) >= MinPhoneLength && len(phone) <= MaxPhoneLength && phonePattern.MatchString(phone)
}

func IsValidAvatarURL(avatarURL string) bool {
	avatarURL = strings.TrimSpace(avatarURL)
	if avatarURL == "" {
		return true
	}
	if len(avatarURL) > MaxAvatarURLLength {
		return false
	}

	parsed, err := url.Parse(avatarURL)
	if err != nil {
		return false
	}

	return (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}

func IsValidBio(bio string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(bio)) <= MaxBioLength
}
