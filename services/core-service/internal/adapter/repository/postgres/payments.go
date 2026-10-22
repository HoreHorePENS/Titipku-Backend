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

type paymentFunctionResult struct {
	PaymentID uuid.UUID `json:"payment_id"`
	Duplicate bool      `json:"duplicate"`
	IsVoid    bool      `json:"is_void"`
}

func (s *Store) CreatePayment(
	ctx context.Context,
	businessID, memberID uuid.UUID,
	payload []byte,
	requestHash string,
) (*domain.Payment, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT record_payment($1, $2, $3::JSONB, $4)
	`, businessID, memberID, string(payload), requestHash).Scan(&raw)
	if err != nil {
		if isUniqueViolation(err) {
			if existing, lookupErr := s.paymentByClientRequestID(ctx, businessID, payload, requestHash); lookupErr == nil {
				return existing, nil
			} else if !errors.Is(lookupErr, domain.ErrNotFound) {
				return nil, lookupErr
			}
		}
		if mapped := mapFunctionError(err); mapped != err {
			return nil, mapped
		}
		return nil, fmt.Errorf("record payment: %w", err)
	}

	var result paymentFunctionResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode record payment result: %w", err)
	}

	return s.PaymentByID(ctx, businessID, result.PaymentID)
}

func (s *Store) paymentByClientRequestID(
	ctx context.Context,
	businessID uuid.UUID,
	payload []byte,
	requestHash string,
) (*domain.Payment, error) {
	var request struct {
		ClientRequestID uuid.UUID `json:"client_request_id"`
	}
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, fmt.Errorf("decode payment request: %w", err)
	}
	if request.ClientRequestID == uuid.Nil {
		return nil, domain.ErrNotFound
	}

	var (
		paymentID    uuid.UUID
		existingHash string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, request_hash
		FROM payments
		WHERE business_id = $1 AND client_request_id = $2
	`, businessID, request.ClientRequestID).Scan(&paymentID, &existingHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find payment by idempotency key: %w", err)
	}
	if existingHash != "" && requestHash != "" && existingHash != requestHash {
		return nil, domain.ErrIdempotencyMismatch
	}

	return s.PaymentByID(ctx, businessID, paymentID)
}

func (s *Store) PaymentByID(ctx context.Context, businessID, paymentID uuid.UUID) (*domain.Payment, error) {
	var payment domain.Payment
	err := s.pool.QueryRow(ctx, `
		SELECT
			p.id, p.business_id, p.partner_id, partner.name, p.member_id,
			p.amount, p.method, p.paid_at, p.is_void, p.created_at
		FROM payments p
		JOIN partners partner ON partner.id = p.partner_id
		WHERE p.business_id = $1 AND p.id = $2
	`, businessID, paymentID).Scan(
		&payment.ID,
		&payment.BusinessID,
		&payment.PartnerID,
		&payment.PartnerName,
		&payment.MemberID,
		&payment.Amount,
		&payment.Method,
		&payment.PaidAt,
		&payment.IsVoid,
		&payment.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find payment: %w", err)
	}
	return &payment, nil
}

func (s *Store) VoidPayment(ctx context.Context, businessID, memberID, paymentID uuid.UUID) (*domain.Payment, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT void_payment($1, $2, $3)
	`, businessID, memberID, paymentID).Scan(&raw)
	if err != nil {
		if mapped := mapFunctionError(err); mapped != err {
			return nil, mapped
		}
		return nil, fmt.Errorf("void payment: %w", err)
	}

	return s.PaymentByID(ctx, businessID, paymentID)
}
