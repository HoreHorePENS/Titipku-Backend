package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

const invoiceSelect = `
	SELECT
		i.id, i.business_id, b.name, b.owner_name, b.whatsapp, b.address,
		i.partner_id, p.name, p.owner_name, p.whatsapp,
		i.visit_id, v.number, v.visited_at, v.public_token, v.received_by,
		i.total, i.paid, i.status, i.issued_at, 0::BIGINT AS total
	FROM invoices i
	JOIN businesses b ON b.id = i.business_id
	JOIN partners p ON p.id = i.partner_id
	JOIN visits v ON v.id = i.visit_id
`

func (s *Store) ListInvoices(
	ctx context.Context,
	businessID uuid.UUID,
	status string,
	partnerID *uuid.UUID,
	limit, offset int,
) ([]domain.Invoice, int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			i.id, i.business_id, b.name, b.owner_name, b.whatsapp, b.address,
			i.partner_id, p.name, p.owner_name, p.whatsapp,
			i.visit_id, v.number, v.visited_at, v.public_token, v.received_by,
			i.total, i.paid, i.status, i.issued_at,
			COUNT(*) OVER() AS total
		FROM invoices i
		JOIN businesses b ON b.id = i.business_id
		JOIN partners p ON p.id = i.partner_id
		JOIN visits v ON v.id = i.visit_id
		WHERE i.business_id = $1
		  AND ($2 = 'all' OR i.status = $2)
		  AND ($3::UUID IS NULL OR i.partner_id = $3)
		ORDER BY i.issued_at DESC, i.id DESC
		LIMIT $4 OFFSET $5
	`, businessID, status, partnerID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list invoices: %w", err)
	}
	defer rows.Close()

	invoices := make([]domain.Invoice, 0)
	var total int64
	for rows.Next() {
		invoice, count, err := scanInvoice(rows)
		if err != nil {
			return nil, 0, err
		}
		total = count
		invoices = append(invoices, *invoice)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate invoices: %w", err)
	}
	return invoices, total, nil
}

func (s *Store) InvoiceByID(ctx context.Context, businessID, invoiceID uuid.UUID) (*domain.Invoice, error) {
	invoice, _, err := scanInvoice(s.pool.QueryRow(ctx, invoiceSelect+`
		WHERE i.business_id = $1 AND i.id = $2
	`, businessID, invoiceID))
	if err != nil {
		return nil, err
	}

	items, err := s.visitItemsByInvoice(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	invoice.Items = items
	return invoice, nil
}

func (s *Store) PublicInvoice(ctx context.Context, token string) (*domain.Invoice, error) {
	invoice, _, err := scanInvoice(s.pool.QueryRow(ctx, invoiceSelect+`
		WHERE v.public_token = $1
	`, token))
	if err != nil {
		return nil, err
	}

	items, err := s.visitItemsByInvoice(ctx, invoice.ID)
	if err != nil {
		return nil, err
	}
	invoice.Items = items
	return invoice, nil
}

func scanInvoice(row pgx.Row) (*domain.Invoice, int64, error) {
	var (
		invoice domain.Invoice
		total   int64
	)
	err := row.Scan(
		&invoice.ID,
		&invoice.BusinessID,
		&invoice.BusinessName,
		&invoice.BusinessOwner,
		&invoice.BusinessWhatsApp,
		&invoice.BusinessAddress,
		&invoice.PartnerID,
		&invoice.PartnerName,
		&invoice.PartnerOwner,
		&invoice.PartnerWhatsApp,
		&invoice.VisitID,
		&invoice.VisitNumber,
		&invoice.VisitDate,
		&invoice.PublicToken,
		&invoice.ReceivedBy,
		&invoice.Total,
		&invoice.Paid,
		&invoice.Status,
		&invoice.IssuedAt,
		&total,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, domain.ErrNotFound
		}
		return nil, 0, fmt.Errorf("scan invoice: %w", err)
	}
	return &invoice, total, nil
}
