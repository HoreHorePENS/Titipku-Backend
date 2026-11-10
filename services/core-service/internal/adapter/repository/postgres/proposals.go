package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

const proposalQuery = `
	SELECT
		cp.id, cp.store_id, cp.business_id, cp.direction, cp.status,
		cp.message, cp.shelf_location, cp.estimated_capacity, cp.created_by_user_id,
		cp.responded_at, cp.created_at, cp.updated_at,
		s.name, s.category, s.address, s.whatsapp, s.photo_url, su.full_name, s.latitude, s.longitude,
		b.name, b.owner_name, b.whatsapp, b.address
	FROM consignment_proposals cp
	JOIN stores s ON s.id = cp.store_id
	JOIN users su ON su.id = s.owner_user_id
	JOIN businesses b ON b.id = cp.business_id
`

func scanProposal(row pgx.Row) (*domain.Proposal, error) {
	var p domain.Proposal
	err := row.Scan(
		&p.ID, &p.StoreID, &p.BusinessID, &p.Direction, &p.Status,
		&p.Message, &p.ShelfLocation, &p.EstimatedCapacity, &p.CreatedByUserID,
		&p.RespondedAt, &p.CreatedAt, &p.UpdatedAt,
		&p.StoreName, &p.StoreCategory, &p.StoreAddress, &p.StoreWhatsApp, &p.StorePhotoURL,
		&p.StoreOwnerName, &p.StoreLatitude, &p.StoreLongitude,
		&p.BusinessName, &p.BusinessOwnerName, &p.BusinessWhatsApp, &p.BusinessAddress,
	)
	if err != nil {
		return nil, err
	}
	p.Items = []domain.ProposalItem{}
	return &p, nil
}

// CreateProposal menyimpan proposal baru beserta item rencana titipan.
func (s *Store) CreateProposal(ctx context.Context, proposal domain.Proposal) (*domain.Proposal, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin proposal: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var alreadyPartners bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM partners
			WHERE business_id = $1 AND store_id = $2 AND is_active
		)
	`, proposal.BusinessID, proposal.StoreID).Scan(&alreadyPartners); err != nil {
		return nil, fmt.Errorf("check partnership: %w", err)
	}
	if alreadyPartners {
		return nil, domain.ErrAlreadyPartners
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO consignment_proposals (
			id, store_id, business_id, direction, status, message, shelf_location,
			estimated_capacity, created_by_user_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8, $9, $9)
	`,
		proposal.ID, proposal.StoreID, proposal.BusinessID, proposal.Direction,
		proposal.Message, proposal.ShelfLocation, proposal.EstimatedCapacity,
		proposal.CreatedByUserID, proposal.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrProposalPending
		}
		if isForeignKeyViolation(err) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("insert proposal: %w", err)
	}

	for _, item := range proposal.Items {
		// Produk wajib milik bisnis penitip dan masih aktif.
		tag, err := tx.Exec(ctx, `
			INSERT INTO consignment_proposal_items (proposal_id, product_id, qty)
			SELECT $1, p.id, $4
			FROM products p
			WHERE p.id = $2 AND p.business_id = $3 AND p.is_active
		`, proposal.ID, item.ProductID, proposal.BusinessID, item.Qty)
		if err != nil {
			if isUniqueViolation(err) {
				return nil, domain.ErrConflict
			}
			return nil, fmt.Errorf("insert proposal item: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil, domain.ErrNotFound
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit proposal: %w", err)
	}
	return s.ProposalByID(ctx, proposal.ID)
}

// ProposalByID mengembalikan proposal lengkap dengan item.
func (s *Store) ProposalByID(ctx context.Context, proposalID uuid.UUID) (*domain.Proposal, error) {
	proposal, err := scanProposal(s.pool.QueryRow(ctx, proposalQuery+` WHERE cp.id = $1`, proposalID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("proposal by id: %w", err)
	}
	list := []domain.Proposal{*proposal}
	if err := s.attachProposalItems(ctx, list); err != nil {
		return nil, err
	}
	return &list[0], nil
}

// ListProposals mengembalikan proposal milik satu toko atau satu bisnis, terbaru dulu.
func (s *Store) ListProposals(ctx context.Context, filter domain.ProposalFilter) ([]domain.Proposal, error) {
	conditions := []string{}
	args := []any{}
	if filter.StoreID != nil {
		args = append(args, *filter.StoreID)
		conditions = append(conditions, fmt.Sprintf("cp.store_id = $%d", len(args)))
	}
	if filter.BusinessID != nil {
		args = append(args, *filter.BusinessID)
		conditions = append(conditions, fmt.Sprintf("cp.business_id = $%d", len(args)))
	}
	if len(conditions) == 0 {
		return nil, fmt.Errorf("list proposals: store or business scope is required")
	}
	if filter.Direction != "" {
		args = append(args, filter.Direction)
		conditions = append(conditions, fmt.Sprintf("cp.direction = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("cp.status = $%d", len(args)))
	}

	rows, err := s.pool.Query(ctx,
		proposalQuery+" WHERE "+strings.Join(conditions, " AND ")+" ORDER BY cp.created_at DESC LIMIT 500",
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list proposals: %w", err)
	}
	defer rows.Close()

	proposals := make([]domain.Proposal, 0)
	for rows.Next() {
		proposal, err := scanProposal(rows)
		if err != nil {
			return nil, fmt.Errorf("scan proposal: %w", err)
		}
		proposals = append(proposals, *proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proposals: %w", err)
	}
	if err := s.attachProposalItems(ctx, proposals); err != nil {
		return nil, err
	}
	return proposals, nil
}

func (s *Store) attachProposalItems(ctx context.Context, proposals []domain.Proposal) error {
	if len(proposals) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(proposals))
	index := make(map[uuid.UUID]int, len(proposals))
	for i, proposal := range proposals {
		ids = append(ids, proposal.ID)
		index[proposal.ID] = i
	}

	rows, err := s.pool.Query(ctx, `
		SELECT i.proposal_id, p.id, p.name, p.photo_path, p.consign_price, p.retail_price,
		       p.shelf_life_days, i.qty
		FROM consignment_proposal_items i
		JOIN products p ON p.id = i.product_id
		WHERE i.proposal_id = ANY($1)
		ORDER BY p.name
	`, ids)
	if err != nil {
		return fmt.Errorf("list proposal items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var proposalID uuid.UUID
		var item domain.ProposalItem
		if err := rows.Scan(
			&proposalID, &item.ProductID, &item.ProductName, &item.PhotoPath,
			&item.ConsignPrice, &item.RetailPrice, &item.ShelfLifeDays, &item.Qty,
		); err != nil {
			return fmt.Errorf("scan proposal item: %w", err)
		}
		if i, ok := index[proposalID]; ok {
			proposals[i].Items = append(proposals[i].Items, item)
		}
	}
	return rows.Err()
}

// ResolveProposal mengubah status proposal pending atau memutus proposal accepted.
// Bila disetujui, toko sekaligus dijadikan partner aktif bisnis penitip dalam transaksi yang sama.
// Bila dibatalkan/diputus dari status accepted, partner terkait dinonaktifkan (is_active = FALSE).
// Jika items tidak kosong, item proposal disesuaikan dengan item yang disetujui.
func (s *Store) ResolveProposal(ctx context.Context, proposalID uuid.UUID, status string, items []domain.ProposalItem) (*domain.Proposal, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin resolve proposal: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var storeID, businessID uuid.UUID
	var prevStatus string
	err = tx.QueryRow(ctx, `
		SELECT status, store_id, business_id
		FROM consignment_proposals
		WHERE id = $1
	`, proposalID).Scan(&prevStatus, &storeID, &businessID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("lookup proposal: %w", err)
	}

	if prevStatus != domain.ProposalPending && !(prevStatus == domain.ProposalAccepted && status == domain.ProposalCancelled) {
		return nil, domain.ErrProposalNotPending
	}

	_, err = tx.Exec(ctx, `
		UPDATE consignment_proposals
		SET status = $2, responded_at = CASE WHEN $2 = 'accepted' OR responded_at IS NULL THEN now() ELSE responded_at END,
		    updated_at = now()
		WHERE id = $1
	`, proposalID, status)
	if err != nil {
		return nil, fmt.Errorf("update proposal status: %w", err)
	}

	if status == domain.ProposalAccepted {
		if len(items) > 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM consignment_proposal_items WHERE proposal_id = $1`, proposalID); err != nil {
				return nil, fmt.Errorf("delete proposal items for partial approval: %w", err)
			}
			for _, item := range items {
				tag, err := tx.Exec(ctx, `
					INSERT INTO consignment_proposal_items (proposal_id, product_id, qty)
					SELECT $1, p.id, $4
					FROM products p
					WHERE p.id = $2 AND p.business_id = $3 AND p.is_active
				`, proposalID, item.ProductID, businessID, item.Qty)
				if err != nil {
					return nil, fmt.Errorf("insert approved proposal item: %w", err)
				}
				if tag.RowsAffected() == 0 {
					return nil, domain.ErrNotFound
				}
			}
		}

		if err := linkStorePartner(ctx, tx, businessID, storeID); err != nil {
			return nil, err
		}
	} else if prevStatus == domain.ProposalAccepted && status == domain.ProposalCancelled {
		// Putus kemitraan: nonaktifkan partner
		_, err = tx.Exec(ctx, `
			UPDATE partners
			SET is_active = FALSE, updated_at = now()
			WHERE business_id = $1 AND store_id = $2
		`, businessID, storeID)
		if err != nil {
			return nil, fmt.Errorf("deactivate partner on termination: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit resolve proposal: %w", err)
	}
	return s.ProposalByID(ctx, proposalID)
}

// SetProposalItems mengisi atau memperbarui daftar produk yang diajukan dalam proposal.
func (s *Store) SetProposalItems(ctx context.Context, proposalID uuid.UUID, items []domain.ProposalItem, direction string) (*domain.Proposal, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin set proposal items: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var businessID uuid.UUID
	var status string
	err = tx.QueryRow(ctx, `
		SELECT business_id, status FROM consignment_proposals WHERE id = $1
	`, proposalID).Scan(&businessID, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("lookup proposal for items: %w", err)
	}
	if status != domain.ProposalPending {
		return nil, domain.ErrProposalNotPending
	}

	if direction != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE consignment_proposals
			SET direction = $2, updated_at = now()
			WHERE id = $1
		`, proposalID, direction); err != nil {
			return nil, fmt.Errorf("update proposal direction: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM consignment_proposal_items WHERE proposal_id = $1`, proposalID); err != nil {
		return nil, fmt.Errorf("clear existing proposal items: %w", err)
	}

	for _, item := range items {
		tag, err := tx.Exec(ctx, `
			INSERT INTO consignment_proposal_items (proposal_id, product_id, qty)
			SELECT $1, p.id, $4
			FROM products p
			WHERE p.id = $2 AND p.business_id = $3 AND p.is_active
		`, proposalID, item.ProductID, businessID, item.Qty)
		if err != nil {
			return nil, fmt.Errorf("insert proposal item: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil, domain.ErrNotFound
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit proposal items: %w", err)
	}
	return s.ProposalByID(ctx, proposalID)
}

// linkStorePartner memastikan toko menjadi partner aktif bisnis:
// pakai partner yang sudah tertaut ke toko, atau tautkan partner manual
// dengan nomor WhatsApp yang sama, atau buat partner baru dari data toko.
func linkStorePartner(ctx context.Context, tx pgx.Tx, businessID, storeID uuid.UUID) error {
	var name, ownerName, whatsapp, address string
	if err := tx.QueryRow(ctx, `
		SELECT s.name, u.full_name, s.whatsapp, s.address
		FROM stores s JOIN users u ON u.id = s.owner_user_id
		WHERE s.id = $1
	`, storeID).Scan(&name, &ownerName, &whatsapp, &address); err != nil {
		return fmt.Errorf("load store for partner: %w", err)
	}
	ownerName = fitOwnerName(ownerName, name)

	tag, err := tx.Exec(ctx, `
		UPDATE partners SET is_active = TRUE, updated_at = now()
		WHERE business_id = $1 AND store_id = $2
	`, businessID, storeID)
	if err != nil {
		return fmt.Errorf("reactivate store partner: %w", err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	tag, err = tx.Exec(ctx, `
		UPDATE partners SET store_id = $2, is_active = TRUE, updated_at = now()
		WHERE business_id = $1 AND whatsapp = $3 AND store_id IS NULL
	`, businessID, storeID, whatsapp)
	if err != nil {
		return fmt.Errorf("link existing partner: %w", err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO partners (id, business_id, name, owner_name, whatsapp, address, store_id, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE, now(), now())
	`, uuid.New(), businessID, name, ownerName, whatsapp, address, storeID)
	if err != nil {
		if isUniqueViolation(err) {
			// Nomor WhatsApp sudah dipakai partner lain yang tertaut ke toko berbeda.
			return domain.ErrWhatsAppTaken
		}
		return fmt.Errorf("create store partner: %w", err)
	}
	return nil
}

// fitOwnerName menyesuaikan nama pemilik dengan batas kolom partners.owner_name (2–100).
func fitOwnerName(ownerName, fallback string) string {
	name := strings.TrimSpace(ownerName)
	if utf8.RuneCountInString(name) < 2 {
		name = strings.TrimSpace(fallback)
	}
	if utf8.RuneCountInString(name) > 100 {
		name = string([]rune(name)[:100])
	}
	return name
}
