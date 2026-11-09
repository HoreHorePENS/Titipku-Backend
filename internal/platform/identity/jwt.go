package identity

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	Email       string      `json:"email"`
	Name        string      `json:"name"`
	Role        Role        `json:"role"`
	AccountType AccountType `json:"account_type,omitempty"`
	BusinessID  string      `json:"business_id,omitempty"`
	MemberID    string      `json:"member_id,omitempty"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

func NewTokenManager(secret, issuer string, ttl time.Duration) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("jwt secret must contain at least 32 characters")
	}
	if issuer == "" {
		return nil, fmt.Errorf("jwt issuer is required")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("jwt ttl must be positive")
	}

	return &TokenManager{
		secret: []byte(secret),
		issuer: issuer,
		ttl:    ttl,
		now:    time.Now,
	}, nil
}

func (m *TokenManager) Issue(userID uuid.UUID, email, name string, role Role) (string, time.Time, error) {
	return m.IssueScoped(userID, email, name, role, AccountTypePenitip, uuid.Nil, uuid.Nil)
}

func (m *TokenManager) IssueScoped(
	userID uuid.UUID,
	email, name string,
	role Role,
	accountType AccountType,
	businessID, memberID uuid.UUID,
) (string, time.Time, error) {
	now := m.now()
	expiresAt := now.Add(m.ttl)

	claims := Claims{
		Email:       email,
		Name:        name,
		Role:        role,
		AccountType: accountType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	if businessID != uuid.Nil {
		claims.BusinessID = businessID.String()
	}
	if memberID != uuid.Nil {
		claims.MemberID = memberID.String()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}

	return signed, expiresAt, nil
}

func (m *TokenManager) Verify(rawToken string) (Principal, error) {
	parsed, err := jwt.ParseWithClaims(
		rawToken,
		&Claims{},
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return m.secret, nil
		},
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil || !parsed.Valid {
		return Principal{}, ErrInvalidToken
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok {
		return Principal{}, ErrInvalidToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil || !claims.Role.Valid() {
		return Principal{}, ErrInvalidToken
	}

	// Token lama tanpa claim account_type diperlakukan sebagai penitip.
	accountType := claims.AccountType
	if accountType == "" {
		accountType = AccountTypePenitip
	}
	if !accountType.Valid() {
		return Principal{}, ErrInvalidToken
	}

	var businessID, memberID uuid.UUID
	if claims.BusinessID != "" {
		businessID, err = uuid.Parse(claims.BusinessID)
		if err != nil {
			return Principal{}, ErrInvalidToken
		}
	}
	if claims.MemberID != "" {
		memberID, err = uuid.Parse(claims.MemberID)
		if err != nil {
			return Principal{}, ErrInvalidToken
		}
	}

	return Principal{
		UserID:      userID,
		Email:       claims.Email,
		Name:        claims.Name,
		Role:        claims.Role,
		AccountType: accountType,
		BusinessID:  businessID,
		MemberID:    memberID,
	}, nil
}
