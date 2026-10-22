package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

const visitSelect = `
	SELECT
		v.id, v.business_id, b.name,
		v.partner_id, p.name, p.owner_name, p.whatsapp,
		v.member_id, v.number, v.public_token, v.visited_at, v.received_by,
		COALESCE(i.total, 0), v.created_at, 0::BIGINT AS total
	FROM visits v
	JOIN businesses b ON b.id = v.business_id
	JOIN partners p ON p.id = v.partner_id
	LEFT JOIN invoices i ON i.visit_id = v.id
`

type visitFunctionResult struct {
	VisitID     uuid.UUID  `json:"visit_id"`
	InvoiceID   *uuid.UUID `json:"invoice_id"`
	Number      string     `json:"number"`
	PublicToken string     `json:"public_token"`
	Total       int64      `json:"total"`
	Duplicate   bool       `json:"duplicate"`
}

func (s *Store) CreateVisit(ctx context.Context, businessID, memberID uuid.UUID, payload []byte, requestHash string) (*domain.VisitResult, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT record_visit($1, $2, $3::JSONB, $4)
	`, businessID, memberID, string(payload), requestHash).Scan(&raw)
	if err != nil {
		if isUniqueViolation(err) {
			if existing, lookupErr := s.visitByClientRequestID(ctx, businessID, payload, requestHash); lookupErr == nil {
				return existing, nil
			} else if !errors.Is(lookupErr, domain.ErrNotFound) {
				return nil, lookupErr
			}
		}
		if mapped := mapFunctionError(err); mapped != err {
			return nil, mapped
		}
		return nil, fmt.Errorf("record visit: %w", err)
	}

	var result visitFunctionResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode record visit result: %w", err)
	}

	var invoiceID uuid.UUID
	if result.InvoiceID != nil {
		invoiceID = *result.InvoiceID
	}

	return &domain.VisitResult{
		ID:          result.VisitID,
		InvoiceID:   invoiceID,
		Number:      result.Number,
		PublicToken: result.PublicToken,
		Total:       result.Total,
		Duplicate:   result.Duplicate,
	}, nil
}

func (s *Store) visitByClientRequestID(
	ctx context.Context,
	businessID uuid.UUID,
	payload []byte,
	requestHash string,
) (*domain.VisitResult, error) {
	var request struct {
		ClientRequestID uuid.UUID `json:"client_request_id"`
	}
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, fmt.Errorf("decode visit request: %w", err)
	}
	if request.ClientRequestID == uuid.Nil {
		return nil, domain.ErrNotFound
	}

	var (
		visitID      uuid.UUID
		invoiceID    *uuid.UUID
		existingHash string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT v.id, i.id, v.request_hash
		FROM visits v
		LEFT JOIN invoices i ON i.visit_id = v.id
		WHERE v.business_id = $1 AND v.client_request_id = $2
	`, businessID, request.ClientRequestID).Scan(&visitID, &invoiceID, &existingHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find visit by idempotency key: %w", err)
	}
	if existingHash != "" && requestHash != "" && existingHash != requestHash {
		return nil, domain.ErrIdempotencyMismatch
	}

	result := &domain.VisitResult{ID: visitID, Duplicate: true}
	if invoiceID != nil {
		result.InvoiceID = *invoiceID
	}
	return result, nil
}

func (s *Store) ListVisits(ctx context.Context, businessID uuid.UUID, partnerID *uuid.UUID, limit, offset int) ([]domain.Visit, int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			v.id, v.business_id, b.name,
			v.partner_id, p.name, p.owner_name, p.whatsapp,
			v.member_id, v.number, v.public_token, v.visited_at, v.received_by,
			COALESCE(i.total, 0), v.created_at,
			COUNT(*) OVER() AS total
		FROM visits v
		JOIN businesses b ON b.id = v.business_id
		JOIN partners p ON p.id = v.partner_id
		LEFT JOIN invoices i ON i.visit_id = v.id
		WHERE v.business_id = $1
		  AND ($2::UUID IS NULL OR v.partner_id = $2)
		ORDER BY v.visited_at DESC, v.id DESC
		LIMIT $3 OFFSET $4
	`, businessID, partnerID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list visits: %w", err)
	}
	defer rows.Close()

	visits := make([]domain.Visit, 0)
	var total int64
	for rows.Next() {
		visit, count, err := scanVisit(rows)
		if err != nil {
			return nil, 0, err
		}
		total = count
		visits = append(visits, *visit)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate visits: %w", err)
	}
	return visits, total, nil
}

func (s *Store) VisitByID(ctx context.Context, businessID, visitID uuid.UUID) (*domain.Visit, error) {
	visit, _, err := scanVisit(s.pool.QueryRow(ctx, visitSelect+`
		WHERE v.business_id = $1 AND v.id = $2
	`, businessID, visitID))
	if err != nil {
		return nil, err
	}

	items, err := s.visitItems(ctx, visitID)
	if err != nil {
		return nil, err
	}
	visit.Items = items
	return visit, nil
}

func scanVisit(row pgx.Row) (*domain.Visit, int64, error) {
	var (
		visit domain.Visit
		total int64
	)
	err := row.Scan(
		&visit.ID,
		&visit.BusinessID,
		&visit.BusinessName,
		&visit.PartnerID,
		&visit.PartnerName,
		&visit.PartnerOwner,
		&visit.PartnerWhatsApp,
		&visit.MemberID,
		&visit.Number,
		&visit.PublicToken,
		&visit.VisitedAt,
		&visit.ReceivedBy,
		&visit.Total,
		&visit.CreatedAt,
		&total,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, domain.ErrNotFound
		}
		return nil, 0, fmt.Errorf("scan visit: %w", err)
	}
	return &visit, total, nil
}

func (s *Store) visitItems(ctx context.Context, visitID uuid.UUID) ([]domain.VisitItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			id, product_id, product_name, prev_stock, remaining, returned,
			new_drop, sold, stock_after, unit_price, subtotal
		FROM visit_items
		WHERE visit_id = $1
		ORDER BY product_name, id
	`, visitID)
	if err != nil {
		return nil, fmt.Errorf("list visit items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.VisitItem, 0)
	for rows.Next() {
		var item domain.VisitItem
		if err := rows.Scan(
			&item.ID,
			&item.ProductID,
			&item.ProductName,
			&item.PrevStock,
			&item.Remaining,
			&item.Returned,
			&item.NewDrop,
			&item.Sold,
			&item.StockAfter,
			&item.UnitPrice,
			&item.Subtotal,
		); err != nil {
			return nil, fmt.Errorf("scan visit item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate visit items: %w", err)
	}
	return items, nil
}

func (s *Store) visitItemsByInvoice(ctx context.Context, invoiceID uuid.UUID) ([]domain.VisitItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			vi.id, vi.product_id, vi.product_name, vi.prev_stock, vi.remaining,
			vi.returned, vi.new_drop, vi.sold, vi.stock_after, vi.unit_price, vi.subtotal
		FROM visit_items vi
		JOIN invoices i ON i.visit_id = vi.visit_id
		WHERE i.id = $1
		ORDER BY vi.product_name, vi.id
	`, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("list invoice items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.VisitItem, 0)
	for rows.Next() {
		var item domain.VisitItem
		if err := rows.Scan(
			&item.ID,
			&item.ProductID,
			&item.ProductName,
			&item.PrevStock,
			&item.Remaining,
			&item.Returned,
			&item.NewDrop,
			&item.Sold,
			&item.StockAfter,
			&item.UnitPrice,
			&item.Subtotal,
		); err != nil {
			return nil, fmt.Errorf("scan invoice item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invoice items: %w", err)
	}
	return items, nil
}
