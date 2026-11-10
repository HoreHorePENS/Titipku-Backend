package domain

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrNotFound                 = errors.New("not found")
	ErrEmailTaken               = errors.New("email is already registered")
	ErrWhatsAppTaken            = errors.New("whatsapp number is already registered")
	ErrConflict                 = errors.New("conflict")
	ErrIdempotencyMismatch      = errors.New("idempotency key was reused with a different payload")
	ErrStockMismatch            = errors.New("stock does not match")
	ErrPaymentExceedsReceivable = errors.New("payment exceeds partner receivable")
	ErrPaymentAllocation        = errors.New("payment allocation failed")
	ErrInconsistentData         = errors.New("inconsistent data")
	ErrStoreNotOwned            = errors.New("store is not owned by this account")
	ErrStoreExists              = errors.New("account already has a store")
	ErrProposalPending          = errors.New("a pending proposal already exists for this store and business")
	ErrAlreadyPartners          = errors.New("store and business are already partners")
	ErrProposalNotPending       = errors.New("proposal is no longer pending")
)

const (
	MinPasswordLength = 8
	MaxPasswordLength = 72
)

var emailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
var whatsappPattern = regexp.MustCompile(`^62[0-9]{8,15}$`)

type User struct {
	ID           uuid.UUID
	Email        string
	FullName     string
	PasswordHash string
	Role         string
	// AccountType membedakan persona: "penitip" (default) atau "pemilik_toko".
	AccountType string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Business struct {
	ID            uuid.UUID
	Name          string
	OwnerName     string
	WhatsApp      *string
	Address       *string
	VisitSequence int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Member struct {
	ID         uuid.UUID
	BusinessID uuid.UUID
	UserID     uuid.UUID
	Role       string
	IsActive   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Account struct {
	User     User
	Member   Member
	Business Business
	// Store diisi hanya untuk akun pemilik_toko.
	Store *Store
}

type Product struct {
	ID            uuid.UUID
	BusinessID    uuid.UUID
	Name          string
	PhotoPath     *string
	ConsignPrice  int64
	RetailPrice   int64
	ShelfLifeDays int32
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Partner struct {
	ID         uuid.UUID
	BusinessID uuid.UUID
	Name       string
	OwnerName  string
	WhatsApp   string
	Address    *string
	// StoreID diisi bila partner ditambahkan dari katalog toko pada peta.
	StoreID    *uuid.UUID
	ProposalID *uuid.UUID // ID proposal kemitraan aktif bila terhubung lewat proposal
	IsActive   bool
	Receivable int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Store adalah toko global pada katalog peta (milik akun pemilik_toko).
type Store struct {
	ID           uuid.UUID
	OwnerUserID  uuid.UUID
	Name         string
	Category     string
	Description  *string
	Address      string
	WhatsApp     string
	Latitude     float64
	Longitude    float64
	PhotoURL     *string
	IsActive     bool
	HasConsigned        bool // sudah dititipkan oleh bisnis penitip yang sedang login (opsional, tergantung konteks)
	HasPendingProposal  bool // ada proposal pending antara toko ini dan bisnis penitip yang sedang login
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// StoreFilter menyaring katalog toko untuk peta.
type StoreFilter struct {
	// MinLat/MaxLat/MinLng/MaxLng adalah viewport peta (opsional).
	MinLat, MaxLat, MinLng, MaxLng *float64
	Category                       string
	Search                         string
	ActiveOnly                     bool
	// ViewerBusinessID diisi agar field HasConsigned ikut dihitung.
	ViewerBusinessID *uuid.UUID
	Limit            int
	Offset           int
}

type VisitItem struct {
	ID          uuid.UUID
	ProductID   uuid.UUID
	ProductName string
	PrevStock   int64
	Remaining   int64
	Returned    int64
	NewDrop     int64
	Sold        int64
	StockAfter  int64
	UnitPrice   int64
	Subtotal    int64
}

type Visit struct {
	ID              uuid.UUID
	BusinessID      uuid.UUID
	BusinessName    string
	PartnerID       uuid.UUID
	PartnerName     string
	PartnerOwner    string
	PartnerWhatsApp string
	MemberID        uuid.UUID
	Number          string
	PublicToken     string
	VisitedAt       time.Time
	ReceivedBy      string
	Total           int64
	// Paid hanya diisi pada nota yang dilihat pemilik toko (pembayaran atas tagihan nota ini).
	Paid            int64
	Items           []VisitItem
	CreatedAt       time.Time
}

type Invoice struct {
	ID               uuid.UUID
	BusinessID       uuid.UUID
	BusinessName     string
	BusinessOwner    string
	BusinessWhatsApp *string
	BusinessAddress  *string
	PartnerID        uuid.UUID
	PartnerName      string
	PartnerOwner     string
	PartnerWhatsApp  string
	VisitID          uuid.UUID
	VisitNumber      string
	VisitDate        time.Time
	PublicToken      string
	ReceivedBy       string
	Total            int64
	Paid             int64
	Status           string
	IssuedAt         time.Time
	Items            []VisitItem
}

type Payment struct {
	ID          uuid.UUID
	BusinessID  uuid.UUID
	PartnerID   uuid.UUID
	PartnerName string
	MemberID    uuid.UUID
	Amount      int64
	Method      string
	PaidAt      time.Time
	IsVoid      bool
	CreatedAt   time.Time
}

type VisitResult struct {
	ID          uuid.UUID
	InvoiceID   uuid.UUID
	Number      string
	PublicToken string
	Total       int64
	Duplicate   bool
}

type Dashboard struct {
	Date             string
	Receivable       int64
	PaidToday        int64
	VisitsToday      int64
	UnpaidInvoices   int64
	ActivePartners   int64
	RecentVisits     []RecentVisit
	PriorityPartners []PriorityPartner
}

type RecentVisit struct {
	ID          uuid.UUID
	Number      string
	PartnerID   uuid.UUID
	PartnerName string
	Total       int64
	Status      string
	VisitedAt   time.Time
}

type PriorityPartner struct {
	ID         uuid.UUID
	Name       string
	OwnerName  string
	WhatsApp   string
	Receivable int64
	LastVisit  *time.Time
}

func NormalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func IsValidEmail(value string) bool {
	return len(value) <= 254 && emailPattern.MatchString(value)
}

func IsValidPassword(value string) bool {
	return len(value) >= MinPasswordLength && len(value) <= MaxPasswordLength
}

func IsValidName(value string) bool {
	length := utf8.RuneCountInString(strings.TrimSpace(value))
	return length >= 2 && length <= 120
}

func NormalizeWhatsApp(value string) (string, bool) {
	var digits strings.Builder
	for _, character := range value {
		if character >= '0' && character <= '9' {
			digits.WriteRune(character)
		}
	}

	normalized := digits.String()
	switch {
	case strings.HasPrefix(normalized, "62"):
	case strings.HasPrefix(normalized, "0"):
		normalized = "62" + normalized[1:]
	case strings.HasPrefix(normalized, "8"):
		normalized = "62" + normalized
	default:
		return "", false
	}

	if !whatsappPattern.MatchString(normalized) {
		return "", false
	}
	return normalized, true
}

func IsValidShelfLife(days int32) bool {
	return days > 0 && days <= 3650
}

func IsValidPaymentMethod(value string) bool {
	switch value {
	case "cash", "transfer", "qris", "other":
		return true
	default:
		return false
	}
}

// StoreCategories adalah daftar kategori toko yang menentukan ikon pada peta.
var StoreCategories = []string{
	"warung", "kelontong", "kafe", "resto", "minimarket", "apotek", "roti", "lainnya",
}

func IsValidStoreCategory(value string) bool {
	for _, category := range StoreCategories {
		if value == category {
			return true
		}
	}
	return false
}

func IsValidDescription(value string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(value)) <= 1000
}

func IsValidAddress(value string) bool {
	length := utf8.RuneCountInString(strings.TrimSpace(value))
	return length >= 3 && length <= 300
}

func IsValidLatitude(value float64) bool {
	return value >= -90 && value <= 90
}

func IsValidLongitude(value float64) bool {
	return value >= -180 && value <= 180
}

func GeneratePublicToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
