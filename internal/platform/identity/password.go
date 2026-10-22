package identity

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const MaxPasswordLength = 72

type PasswordHasher struct {
	cost int
}

func NewPasswordHasher() *PasswordHasher {
	return &PasswordHasher{cost: bcrypt.DefaultCost}
}

func (h *PasswordHasher) Hash(plainPassword string) (string, error) {
	if len(plainPassword) > MaxPasswordLength {
		return "", fmt.Errorf("password exceeds %d bytes", MaxPasswordLength)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), h.cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

func (h *PasswordHasher) Compare(hash, plainPassword string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plainPassword)); err != nil {
		return fmt.Errorf("compare password: %w", err)
	}
	return nil
}
