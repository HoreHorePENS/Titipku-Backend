package domain

import (
	"time"

	"github.com/google/uuid"
)

// PartnerStockItem adalah posisi stok satu produk penitip di satu mitra (toko/warung),
// dihitung dari sisa batch (qty_left) — sumber yang sama dengan pencatatan kunjungan.
type PartnerStockItem struct {
	ProductID     uuid.UUID
	ProductName   string
	PhotoPath     *string
	ConsignPrice  int64
	IsActive      bool
	OnShelf       int64
	NearestExpiry *time.Time
	// EverConsigned bernilai true bila produk pernah dititipkan ke mitra ini.
	EverConsigned bool
}
