package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

type Store interface {
	CreateAccount(ctx context.Context, account domain.Account, passwordHash string) error
	AccountByEmail(ctx context.Context, email string) (*domain.Account, error)
	AccountByUserID(ctx context.Context, userID uuid.UUID) (*domain.Account, error)
	UpdateBusiness(ctx context.Context, business domain.Business) (*domain.Business, error)

	ListProducts(ctx context.Context, businessID uuid.UUID, activeOnly bool) ([]domain.Product, error)
	UpsertProduct(ctx context.Context, product domain.Product) (*domain.Product, error)
	SetProductActive(ctx context.Context, businessID, productID uuid.UUID, active bool) (*domain.Product, error)

	ListStores(ctx context.Context, filter domain.StoreFilter) ([]domain.Store, error)
	StoreByID(ctx context.Context, storeID uuid.UUID, viewerBusinessID *uuid.UUID) (*domain.Store, error)
	StoreByOwnerUserID(ctx context.Context, userID uuid.UUID) (*domain.Store, error)
	CreateStore(ctx context.Context, store domain.Store) (*domain.Store, error)
	UpdateStore(ctx context.Context, store domain.Store) (*domain.Store, error)
	SetStoreActive(ctx context.Context, ownerUserID, storeID uuid.UUID, active bool) (*domain.Store, error)

	SetStorePhoto(ctx context.Context, ownerUserID uuid.UUID, photoURL *string) (*domain.Store, *string, error)
	SetProductPhoto(ctx context.Context, businessID, productID uuid.UUID, photoPath *string) (*domain.Product, *string, error)

	CreateProposal(ctx context.Context, proposal domain.Proposal) (*domain.Proposal, error)
	ProposalByID(ctx context.Context, proposalID uuid.UUID) (*domain.Proposal, error)
	ListProposals(ctx context.Context, filter domain.ProposalFilter) ([]domain.Proposal, error)
	ResolveProposal(ctx context.Context, proposalID uuid.UUID, status string) (*domain.Proposal, error)
	ListProducers(ctx context.Context, filter domain.ProducerFilter) ([]domain.Producer, error)

	ListPartners(ctx context.Context, businessID uuid.UUID, activeOnly bool) ([]domain.Partner, error)
	UpsertPartner(ctx context.Context, partner domain.Partner) (*domain.Partner, error)
	SetPartnerActive(ctx context.Context, businessID, partnerID uuid.UUID, active bool) (*domain.Partner, error)

	CreateVisit(ctx context.Context, businessID, memberID uuid.UUID, payload []byte, requestHash string) (*domain.VisitResult, error)
	ListVisits(ctx context.Context, businessID uuid.UUID, partnerID *uuid.UUID, limit, offset int) ([]domain.Visit, int64, error)
	VisitByID(ctx context.Context, businessID, visitID uuid.UUID) (*domain.Visit, error)

	ListInvoices(ctx context.Context, businessID uuid.UUID, status string, partnerID *uuid.UUID, limit, offset int) ([]domain.Invoice, int64, error)
	InvoiceByID(ctx context.Context, businessID, invoiceID uuid.UUID) (*domain.Invoice, error)
	PublicInvoice(ctx context.Context, token string) (*domain.Invoice, error)

	CreatePayment(ctx context.Context, businessID, memberID uuid.UUID, payload []byte, requestHash string) (*domain.Payment, error)
	PaymentByID(ctx context.Context, businessID, paymentID uuid.UUID) (*domain.Payment, error)
	VoidPayment(ctx context.Context, businessID, memberID, paymentID uuid.UUID) (*domain.Payment, error)

	Dashboard(ctx context.Context, businessID uuid.UUID, day time.Time) (*domain.Dashboard, error)
}

type PasswordHasher interface {
	Hash(plainPassword string) (string, error)
	Compare(hash, plainPassword string) error
}

type TokenIssuer interface {
	IssueScoped(userID uuid.UUID, email, name string, role identity.Role, accountType identity.AccountType, businessID, memberID uuid.UUID) (string, time.Time, error)
}
