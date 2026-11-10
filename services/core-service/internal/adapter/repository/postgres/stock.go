package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

// PartnerStock mengembalikan produk aktif bisnis beserta stok di rak mitra,
// ditambah produk nonaktif yang masih punya sisa stok di mitra tersebut.
func (s *Store) PartnerStock(ctx context.Context, businessID, partnerID uuid.UUID) ([]domain.PartnerStockItem, error) {
	if _, err := s.partnerByID(ctx, businessID, partnerID); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			p.id, p.name, p.photo_path, p.consign_price, p.is_active,
			COALESCE(SUM(b.qty_left), 0)::BIGINT AS on_shelf,
			MIN(b.expires_at) FILTER (WHERE b.qty_left > 0) AS nearest_expiry,
			COUNT(b.id) > 0 AS ever_consigned
		FROM products p
		LEFT JOIN batches b
			ON b.product_id = p.id AND b.business_id = p.business_id AND b.partner_id = $2
		WHERE p.business_id = $1
		GROUP BY p.id, p.name, p.photo_path, p.consign_price, p.is_active
		HAVING p.is_active OR COALESCE(SUM(b.qty_left), 0) > 0
		ORDER BY lower(p.name), p.id
	`, businessID, partnerID)
	if err != nil {
		return nil, fmt.Errorf("partner stock: %w", err)
	}
	defer rows.Close()

	items := make([]domain.PartnerStockItem, 0)
	for rows.Next() {
		var item domain.PartnerStockItem
		if err := rows.Scan(
			&item.ProductID, &item.ProductName, &item.PhotoPath, &item.ConsignPrice, &item.IsActive,
			&item.OnShelf, &item.NearestExpiry, &item.EverConsigned,
		); err != nil {
			return nil, fmt.Errorf("scan partner stock: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate partner stock: %w", err)
	}
	return items, nil
}
