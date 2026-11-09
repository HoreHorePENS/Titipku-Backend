package domain

import (
	"time"

	"github.com/google/uuid"
)

// Arah proposal kemitraan konsinyasi.
const (
	// ProposalStoreToPenitip adalah "tawaran": toko menawarkan tempat ke penitip.
	ProposalStoreToPenitip = "store_to_penitip"
	// ProposalPenitipToStore adalah "pengajuan": penitip mengajukan titip-jual ke toko.
	ProposalPenitipToStore = "penitip_to_store"
)

// Status proposal.
const (
	ProposalPending   = "pending"
	ProposalAccepted  = "accepted"
	ProposalRejected  = "rejected"
	ProposalCancelled = "cancelled"
)

const (
	MaxProposalMessage   = 1000
	MaxShelfLocation     = 120
	MaxEstimatedCapacity = 60
	MaxProposalItems     = 50
	MaxProposalItemQty   = 100000
)

// ProposalItem adalah rencana produk yang akan dititipkan (snapshot dari katalog saat dibaca).
type ProposalItem struct {
	ProductID     uuid.UUID
	ProductName   string
	PhotoPath     *string
	ConsignPrice  int64
	RetailPrice   int64
	ShelfLifeDays int32
	Qty           int32
}

// Proposal adalah tawaran/pengajuan kemitraan antara toko dan bisnis penitip.
type Proposal struct {
	ID                uuid.UUID
	StoreID           uuid.UUID
	BusinessID        uuid.UUID
	Direction         string
	Status            string
	Message           *string
	ShelfLocation     *string
	EstimatedCapacity *string
	CreatedByUserID   uuid.UUID
	RespondedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Items             []ProposalItem

	// Ringkasan pihak toko.
	StoreName      string
	StoreCategory  string
	StoreAddress   string
	StoreWhatsApp  string
	StorePhotoURL  *string
	StoreOwnerName string
	StoreLatitude  float64
	StoreLongitude float64

	// Ringkasan pihak penitip.
	BusinessName      string
	BusinessOwnerName string
	BusinessWhatsApp  *string
	BusinessAddress   *string
}

// ProposalFilter memilih proposal milik satu pihak.
type ProposalFilter struct {
	// Tepat satu dari StoreID atau BusinessID diisi.
	StoreID    *uuid.UUID
	BusinessID *uuid.UUID
	Direction  string
	Status     string
}

// Producer adalah bisnis penitip yang tampil di katalog untuk pemilik toko.
type Producer struct {
	BusinessID   uuid.UUID
	Name         string
	OwnerName    string
	WhatsApp     *string
	Address      *string
	ProductCount int
	Products     []Product
	// Proposal terakhir antara toko yang melihat dan bisnis ini (jika ada).
	LatestProposal *Proposal
	// IsPartner true bila toko ini sudah menjadi partner aktif bisnis tersebut.
	IsPartner bool
}

// ProducerFilter menyaring katalog produsen.
type ProducerFilter struct {
	ViewerStoreID uuid.UUID
	Search        string
	Limit         int
	Offset        int
}
