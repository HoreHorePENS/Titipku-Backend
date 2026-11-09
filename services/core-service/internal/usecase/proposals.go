package usecase

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

// Actor adalah pihak yang memanggil endpoint proposal.
type Actor struct {
	UserID      uuid.UUID
	AccountType identity.AccountType
	// BusinessID terisi untuk penitip (dari token).
	BusinessID uuid.UUID
}

func (a Actor) isStoreOwner() bool { return a.AccountType == identity.AccountTypePemilikToko }

type ProposalItemInput struct {
	ProductID uuid.UUID
	Qty       int32
}

type CreateProposalInput struct {
	// Target: StoreID wajib untuk penitip, BusinessID wajib untuk pemilik toko.
	StoreID           uuid.UUID
	BusinessID        uuid.UUID
	Message           string
	ShelfLocation     string
	EstimatedCapacity string
	Items             []ProposalItemInput
}

type ProposalListInput struct {
	Direction string
	Status    string
}

// ProducerListInput menyaring katalog produsen untuk pemilik toko.
type ProducerListInput struct {
	Search string
	Limit  int
	Offset int
}

func optionalText(value string, max int, field string, fields map[string]string) *string {
	trimmed := strings.TrimSpace(value)
	if utf8.RuneCountInString(trimmed) > max {
		fields[field] = "must be at most " + strconv.Itoa(max) + " characters"
		return nil
	}
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// myStoreID mengembalikan toko milik pemilik toko yang sedang login.
func (a *App) myStoreID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	store, err := a.store.StoreByOwnerUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return uuid.Nil, apperror.NotFound("STORE_NOT_FOUND", "this account does not have a store yet")
		}
		return uuid.Nil, mapStoreError(err, "STORE_LOOKUP_FAILED", "unable to load store")
	}
	return store.ID, nil
}

// CreateProposal membuat tawaran (pemilik toko -> penitip) atau pengajuan (penitip -> toko).
func (a *App) CreateProposal(ctx context.Context, actor Actor, input CreateProposalInput) (*domain.Proposal, error) {
	fields := make(map[string]string)
	proposal := domain.Proposal{
		ID:              uuid.New(),
		Status:          domain.ProposalPending,
		CreatedByUserID: actor.UserID,
		CreatedAt:       a.now(),
	}
	proposal.Message = optionalText(input.Message, domain.MaxProposalMessage, "message", fields)
	proposal.ShelfLocation = optionalText(input.ShelfLocation, domain.MaxShelfLocation, "shelf_location", fields)
	proposal.EstimatedCapacity = optionalText(input.EstimatedCapacity, domain.MaxEstimatedCapacity, "estimated_capacity", fields)

	var err error
	if actor.isStoreOwner() {
		err = a.addressStoreOffer(ctx, actor, input, &proposal, fields)
	} else {
		err = a.addressPenitipRequest(ctx, actor, input, &proposal, fields)
	}
	if err != nil {
		return nil, err
	}

	created, err := a.store.CreateProposal(ctx, proposal)
	if err != nil {
		return nil, mapStoreError(err, "PROPOSAL_CREATE_FAILED", "unable to create proposal")
	}
	return created, nil
}

// addressStoreOffer mengisi arah & pihak untuk tawaran dari pemilik toko ke penitip.
func (a *App) addressStoreOffer(ctx context.Context, actor Actor, input CreateProposalInput, proposal *domain.Proposal, fields map[string]string) error {
	if input.BusinessID == uuid.Nil {
		fields["business_id"] = "is required"
	}
	if len(input.Items) > 0 {
		fields["items"] = "must not be set by store owners"
	}
	if len(fields) > 0 {
		return apperror.Validation(fields)
	}
	storeID, err := a.myStoreID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	proposal.Direction = domain.ProposalStoreToPenitip
	proposal.StoreID = storeID
	proposal.BusinessID = input.BusinessID
	return nil
}

// addressPenitipRequest mengisi arah, pihak, dan item untuk pengajuan penitip ke toko aktif.
func (a *App) addressPenitipRequest(ctx context.Context, actor Actor, input CreateProposalInput, proposal *domain.Proposal, fields map[string]string) error {
	if input.StoreID == uuid.Nil {
		fields["store_id"] = "is required"
	}
	proposal.Items = proposalItems(input.Items, fields)
	if len(fields) > 0 {
		return apperror.Validation(fields)
	}

	store, err := a.store.StoreByID(ctx, input.StoreID, &actor.BusinessID)
	if err != nil {
		return mapStoreError(err, "STORE_LOOKUP_FAILED", "unable to load store")
	}
	if !store.IsActive {
		return apperror.Conflict("STORE_INACTIVE", "store is not accepting consignments right now")
	}
	proposal.Direction = domain.ProposalPenitipToStore
	proposal.StoreID = store.ID
	proposal.BusinessID = actor.BusinessID
	return nil
}

// proposalItems memvalidasi rencana produk: unik, jumlah 1..100000, maksimal 50 item.
func proposalItems(inputs []ProposalItemInput, fields map[string]string) []domain.ProposalItem {
	if len(inputs) > domain.MaxProposalItems {
		fields["items"] = "must contain at most 50 items"
	}
	items := make([]domain.ProposalItem, 0, len(inputs))
	seen := map[uuid.UUID]bool{}
	for _, item := range inputs {
		switch {
		case item.ProductID == uuid.Nil:
			fields["items"] = "must be a valid UUID"
		case seen[item.ProductID]:
			fields["items"] = "must not be duplicated"
		case item.Qty < 1 || item.Qty > domain.MaxProposalItemQty:
			fields["items"] = "must be between 1 and 100000"
		}
		seen[item.ProductID] = true
		items = append(items, domain.ProposalItem{ProductID: item.ProductID, Qty: item.Qty})
	}
	return items
}

// ListProposals mengembalikan proposal yang melibatkan pemanggil (sebagai toko atau penitip).
func (a *App) ListProposals(ctx context.Context, actor Actor, input ProposalListInput) ([]domain.Proposal, error) {
	fields := make(map[string]string)
	switch input.Direction {
	case "", domain.ProposalStoreToPenitip, domain.ProposalPenitipToStore:
	default:
		fields["direction"] = "must be one of store_to_penitip, penitip_to_store"
	}
	switch input.Status {
	case "", domain.ProposalPending, domain.ProposalAccepted, domain.ProposalRejected, domain.ProposalCancelled:
	default:
		fields["status"] = "must be one of pending, accepted, rejected, cancelled"
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	filter := domain.ProposalFilter{Direction: input.Direction, Status: input.Status}
	if actor.isStoreOwner() {
		storeID, err := a.myStoreID(ctx, actor.UserID)
		if err != nil {
			return nil, err
		}
		filter.StoreID = &storeID
	} else {
		businessID := actor.BusinessID
		filter.BusinessID = &businessID
	}

	proposals, err := a.store.ListProposals(ctx, filter)
	if err != nil {
		return nil, mapStoreError(err, "PROPOSAL_LIST_FAILED", "unable to load proposals")
	}
	return proposals, nil
}

// proposalForActor memuat proposal dan memastikan pemanggil adalah salah satu pihaknya.
// Proposal pihak lain dilaporkan sebagai tidak ditemukan.
func (a *App) proposalForActor(ctx context.Context, actor Actor, proposalID uuid.UUID) (*domain.Proposal, bool, error) {
	proposal, err := a.store.ProposalByID(ctx, proposalID)
	if err != nil {
		return nil, false, mapStoreError(err, "PROPOSAL_LOOKUP_FAILED", "unable to load proposal")
	}

	if actor.isStoreOwner() {
		storeID, err := a.myStoreID(ctx, actor.UserID)
		if err != nil {
			return nil, false, err
		}
		if proposal.StoreID != storeID {
			return nil, false, apperror.NotFound("NOT_FOUND", "requested resource was not found")
		}
		return proposal, proposal.Direction == domain.ProposalStoreToPenitip, nil
	}

	if proposal.BusinessID != actor.BusinessID {
		return nil, false, apperror.NotFound("NOT_FOUND", "requested resource was not found")
	}
	return proposal, proposal.Direction == domain.ProposalPenitipToStore, nil
}

// RespondProposal menyetujui atau menolak proposal. Hanya penerima yang boleh.
func (a *App) RespondProposal(ctx context.Context, actor Actor, proposalID uuid.UUID, accept bool) (*domain.Proposal, error) {
	proposal, isInitiator, err := a.proposalForActor(ctx, actor, proposalID)
	if err != nil {
		return nil, err
	}
	if isInitiator {
		return nil, apperror.Forbidden("ONLY_RECIPIENT_CAN_RESPOND", "only the receiving party can accept or reject this proposal")
	}
	if proposal.Status != domain.ProposalPending {
		return nil, apperror.Conflict("PROPOSAL_NOT_PENDING", "proposal is no longer pending")
	}

	status := domain.ProposalRejected
	if accept {
		status = domain.ProposalAccepted
	}
	resolved, err := a.store.ResolveProposal(ctx, proposalID, status)
	if err != nil {
		return nil, mapStoreError(err, "PROPOSAL_UPDATE_FAILED", "unable to update proposal")
	}
	return resolved, nil
}

// CancelProposal membatalkan proposal yang masih pending. Hanya pengirim yang boleh.
func (a *App) CancelProposal(ctx context.Context, actor Actor, proposalID uuid.UUID) (*domain.Proposal, error) {
	proposal, isInitiator, err := a.proposalForActor(ctx, actor, proposalID)
	if err != nil {
		return nil, err
	}
	if !isInitiator {
		return nil, apperror.Forbidden("ONLY_SENDER_CAN_CANCEL", "only the sending party can cancel this proposal")
	}
	if proposal.Status != domain.ProposalPending {
		return nil, apperror.Conflict("PROPOSAL_NOT_PENDING", "proposal is no longer pending")
	}

	resolved, err := a.store.ResolveProposal(ctx, proposalID, domain.ProposalCancelled)
	if err != nil {
		return nil, mapStoreError(err, "PROPOSAL_UPDATE_FAILED", "unable to update proposal")
	}
	return resolved, nil
}

// ListProducers mengembalikan katalog produsen penitip untuk pemilik toko.
func (a *App) ListProducers(ctx context.Context, actor Actor, input ProducerListInput) ([]domain.Producer, error) {
	storeID, err := a.myStoreID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	producers, err := a.store.ListProducers(ctx, domain.ProducerFilter{
		ViewerStoreID: storeID,
		Search:        input.Search,
		Limit:         input.Limit,
		Offset:        input.Offset,
	})
	if err != nil {
		return nil, mapStoreError(err, "PRODUCER_LIST_FAILED", "unable to load producers")
	}
	return producers, nil
}
