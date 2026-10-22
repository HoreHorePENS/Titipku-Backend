package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

var wib = time.FixedZone("WIB", 7*60*60)

func (s *Store) Dashboard(ctx context.Context, businessID uuid.UUID, day time.Time) (*domain.Dashboard, error) {
	result := &domain.Dashboard{Date: day.In(wib).Format("2006-01-02")}

	err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE((
				SELECT SUM(total - paid)
				FROM invoices
				WHERE business_id = $1 AND paid < total
			), 0),
			COALESCE((
				SELECT SUM(amount)
				FROM payments
				WHERE business_id = $1
				  AND NOT is_void
				  AND (paid_at AT TIME ZONE 'Asia/Jakarta')::DATE = ($2 AT TIME ZONE 'Asia/Jakarta')::DATE
			), 0),
			(
				SELECT COUNT(*)
				FROM visits
				WHERE business_id = $1
				  AND (visited_at AT TIME ZONE 'Asia/Jakarta')::DATE = ($2 AT TIME ZONE 'Asia/Jakarta')::DATE
			),
			(
				SELECT COUNT(*)
				FROM invoices
				WHERE business_id = $1 AND paid < total
			),
			(
				SELECT COUNT(*)
				FROM partners
				WHERE business_id = $1 AND is_active
			)
	`, businessID, day).Scan(
		&result.Receivable,
		&result.PaidToday,
		&result.VisitsToday,
		&result.UnpaidInvoices,
		&result.ActivePartners,
	)
	if err != nil {
		return nil, fmt.Errorf("load dashboard totals: %w", err)
	}

	recentRows, err := s.pool.Query(ctx, `
		SELECT
			v.id, v.number, p.id, p.name,
			COALESCE(i.total, 0), COALESCE(i.status, 'paid'), v.visited_at
		FROM visits v
		JOIN partners p ON p.id = v.partner_id
		LEFT JOIN invoices i ON i.visit_id = v.id
		WHERE v.business_id = $1
		ORDER BY v.visited_at DESC, v.id DESC
		LIMIT 5
	`, businessID)
	if err != nil {
		return nil, fmt.Errorf("load recent visits: %w", err)
	}
	defer recentRows.Close()

	for recentRows.Next() {
		var visit domain.RecentVisit
		if err := recentRows.Scan(
			&visit.ID,
			&visit.Number,
			&visit.PartnerID,
			&visit.PartnerName,
			&visit.Total,
			&visit.Status,
			&visit.VisitedAt,
		); err != nil {
			return nil, fmt.Errorf("scan recent visit: %w", err)
		}
		result.RecentVisits = append(result.RecentVisits, visit)
	}
	if err := recentRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent visits: %w", err)
	}

	priorityRows, err := s.pool.Query(ctx, `
		SELECT
			p.id, p.name, p.owner_name, p.whatsapp,
			COALESCE((
				SELECT SUM(i.total - i.paid)
				FROM invoices i
				WHERE i.business_id = p.business_id
				  AND i.partner_id = p.id
				  AND i.paid < i.total
			), 0) AS receivable,
			(
				SELECT MAX(v.visited_at)
				FROM visits v
				WHERE v.business_id = p.business_id
				  AND v.partner_id = p.id
			) AS last_visit
		FROM partners p
		WHERE p.business_id = $1 AND p.is_active
		ORDER BY receivable DESC, last_visit ASC NULLS FIRST, p.name
		LIMIT 5
	`, businessID)
	if err != nil {
		return nil, fmt.Errorf("load priority partners: %w", err)
	}
	defer priorityRows.Close()

	for priorityRows.Next() {
		var (
			partner   domain.PriorityPartner
			lastVisit pgtype.Timestamptz
		)
		if err := priorityRows.Scan(
			&partner.ID,
			&partner.Name,
			&partner.OwnerName,
			&partner.WhatsApp,
			&partner.Receivable,
			&lastVisit,
		); err != nil {
			return nil, fmt.Errorf("scan priority partner: %w", err)
		}
		if lastVisit.Valid {
			value := lastVisit.Time
			partner.LastVisit = &value
		}
		result.PriorityPartners = append(result.PriorityPartners, partner)
	}
	if err := priorityRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate priority partners: %w", err)
	}

	return result, nil
}
