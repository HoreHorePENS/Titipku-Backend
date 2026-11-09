package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

// producerProductLimit membatasi produk yang dikirim per produsen di katalog.
const producerProductLimit = 12

// ListProducers mengembalikan bisnis penitip yang punya minimal satu produk aktif,
// beserta produk aktifnya dan status hubungan dengan toko yang melihat.
func (s *Store) ListProducers(ctx context.Context, filter domain.ProducerFilter) ([]domain.Producer, error) {
	args := []any{filter.ViewerStoreID}
	conditions := []string{
		"u.account_type = 'penitip'",
		"EXISTS (SELECT 1 FROM products ap WHERE ap.business_id = b.id AND ap.is_active)",
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		conditions = append(conditions, fmt.Sprintf(`(
			lower(b.name) LIKE $%[1]d OR lower(b.owner_name) LIKE $%[1]d OR lower(coalesce(b.address, '')) LIKE $%[1]d
			OR EXISTS (SELECT 1 FROM products sp WHERE sp.business_id = b.id AND sp.is_active AND lower(sp.name) LIKE $%[1]d)
		)`, len(args)))
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args = append(args, limit, max(filter.Offset, 0))

	query := fmt.Sprintf(`
		SELECT
			b.id, b.name, b.owner_name, b.whatsapp, b.address,
			(SELECT count(*) FROM products cp WHERE cp.business_id = b.id AND cp.is_active)::int,
			EXISTS (
				SELECT 1 FROM partners pa
				WHERE pa.business_id = b.id AND pa.store_id = $1 AND pa.is_active
			),
			lp.id
		FROM businesses b
		JOIN members m ON m.business_id = b.id AND m.role = 'owner' AND m.is_active
		JOIN users u ON u.id = m.user_id
		LEFT JOIN LATERAL (
			SELECT cp.id FROM consignment_proposals cp
			WHERE cp.store_id = $1 AND cp.business_id = b.id
			ORDER BY cp.created_at DESC
			LIMIT 1
		) lp ON TRUE
		WHERE %s
		ORDER BY b.created_at DESC, b.name
		LIMIT $%d OFFSET $%d
	`, strings.Join(conditions, " AND "), len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list producers: %w", err)
	}
	defer rows.Close()

	producers := make([]domain.Producer, 0)
	latestProposalIDs := map[uuid.UUID]int{}
	for rows.Next() {
		var producer domain.Producer
		var latestProposalID *uuid.UUID
		if err := rows.Scan(
			&producer.BusinessID, &producer.Name, &producer.OwnerName, &producer.WhatsApp,
			&producer.Address, &producer.ProductCount, &producer.IsPartner, &latestProposalID,
		); err != nil {
			return nil, fmt.Errorf("scan producer: %w", err)
		}
		producer.Products = []domain.Product{}
		if latestProposalID != nil {
			latestProposalIDs[*latestProposalID] = len(producers)
		}
		producers = append(producers, producer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate producers: %w", err)
	}
	rows.Close()

	if err := s.attachProducerProducts(ctx, producers); err != nil {
		return nil, err
	}
	for proposalID, i := range latestProposalIDs {
		proposal, err := s.ProposalByID(ctx, proposalID)
		if err != nil {
			return nil, err
		}
		producers[i].LatestProposal = proposal
	}
	return producers, nil
}

func (s *Store) attachProducerProducts(ctx context.Context, producers []domain.Producer) error {
	if len(producers) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(producers))
	index := make(map[uuid.UUID]int, len(producers))
	for i, producer := range producers {
		ids = append(ids, producer.BusinessID)
		index[producer.BusinessID] = i
	}

	rows, err := s.pool.Query(ctx, `
		SELECT `+productColumns+`
		FROM (
			SELECT p.*, row_number() OVER (PARTITION BY p.business_id ORDER BY p.name) AS rn
			FROM products p
			WHERE p.business_id = ANY($1) AND p.is_active
		) p
		WHERE p.rn <= $2
		ORDER BY p.business_id, p.name
	`, ids, producerProductLimit)
	if err != nil {
		return fmt.Errorf("list producer products: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return fmt.Errorf("scan producer product: %w", err)
		}
		if i, ok := index[product.BusinessID]; ok {
			producers[i].Products = append(producers[i].Products, *product)
		}
	}
	return rows.Err()
}

// SetStorePhoto mengganti foto toko milik ownerUserID dan mengembalikan URL lama.
func (s *Store) SetStorePhoto(ctx context.Context, ownerUserID uuid.UUID, photoURL *string) (*domain.Store, *string, error) {
	var storeID uuid.UUID
	var previous *string
	err := s.pool.QueryRow(ctx, `
		WITH old AS (
			SELECT id, photo_url FROM stores WHERE owner_user_id = $1 FOR UPDATE
		)
		UPDATE stores s
		SET photo_url = $2, updated_at = now()
		FROM old
		WHERE s.id = old.id
		RETURNING s.id, old.photo_url
	`, ownerUserID, photoURL).Scan(&storeID, &previous)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.ErrNotFound
		}
		return nil, nil, fmt.Errorf("set store photo: %w", err)
	}
	store, err := s.StoreByID(ctx, storeID, nil)
	return store, previous, err
}

// SetProductPhoto mengganti foto produk milik bisnis dan mengembalikan path lama.
func (s *Store) SetProductPhoto(ctx context.Context, businessID, productID uuid.UUID, photoPath *string) (*domain.Product, *string, error) {
	var previous *string
	err := s.pool.QueryRow(ctx, `
		SELECT photo_path FROM products WHERE id = $1 AND business_id = $2
	`, productID, businessID).Scan(&previous)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.ErrNotFound
		}
		return nil, nil, fmt.Errorf("load product photo: %w", err)
	}

	product, err := scanProduct(s.pool.QueryRow(ctx, `
		UPDATE products
		SET photo_path = $3, updated_at = now()
		WHERE id = $1 AND business_id = $2
		RETURNING `+productColumns,
		productID, businessID, photoPath,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.ErrNotFound
		}
		return nil, nil, fmt.Errorf("set product photo: %w", err)
	}
	return product, previous, nil
}
