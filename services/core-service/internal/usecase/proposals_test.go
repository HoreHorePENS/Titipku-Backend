package usecase

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/internal/platform/imageupload"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

// fakeStore mengimplementasikan sebagian port Store yang dipakai test ini.
type fakeStore struct {
	Store

	stores    map[uuid.UUID]domain.Store // by owner user id
	proposals map[uuid.UUID]*domain.Proposal

	storePhoto    *string
	setPhotoErr   error
	createdInputs []domain.Proposal
}

func newFakeStore() *fakeStore {
	return &fakeStore{stores: map[uuid.UUID]domain.Store{}, proposals: map[uuid.UUID]*domain.Proposal{}}
}

func (f *fakeStore) StoreByOwnerUserID(_ context.Context, userID uuid.UUID) (*domain.Store, error) {
	store, ok := f.stores[userID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &store, nil
}

func (f *fakeStore) StoreByID(_ context.Context, storeID uuid.UUID, _ *uuid.UUID) (*domain.Store, error) {
	for _, store := range f.stores {
		if store.ID == storeID {
			s := store
			return &s, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeStore) CreateProposal(_ context.Context, p domain.Proposal) (*domain.Proposal, error) {
	f.createdInputs = append(f.createdInputs, p)
	f.proposals[p.ID] = &p
	return &p, nil
}

func (f *fakeStore) ProposalByID(_ context.Context, id uuid.UUID) (*domain.Proposal, error) {
	p, ok := f.proposals[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copy := *p
	return &copy, nil
}

func (f *fakeStore) ResolveProposal(_ context.Context, id uuid.UUID, status string, items []domain.ProposalItem) (*domain.Proposal, error) {
	p, ok := f.proposals[id]
	if !ok || p.Status != domain.ProposalPending {
		return nil, domain.ErrProposalNotPending
	}
	p.Status = status
	if len(items) > 0 {
		p.Items = items
	}
	copy := *p
	return &copy, nil
}

func (f *fakeStore) SetProposalItems(_ context.Context, id uuid.UUID, items []domain.ProposalItem, direction string) (*domain.Proposal, error) {
	p, ok := f.proposals[id]
	if !ok || p.Status != domain.ProposalPending {
		return nil, domain.ErrProposalNotPending
	}
	p.Items = items
	if direction != "" {
		p.Direction = direction
	}
	copy := *p
	return &copy, nil
}

func (f *fakeStore) PartnerByStoreID(_ context.Context, _, _ uuid.UUID) (*domain.Partner, error) {
	return nil, domain.ErrNotFound
}

func (f *fakeStore) OwnerMemberByBusinessID(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, domain.ErrNotFound
}

func (f *fakeStore) ListStoreVisits(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.Visit, int64, error) {
	return nil, 0, nil
}

func (f *fakeStore) SetStorePhoto(_ context.Context, userID uuid.UUID, url *string) (*domain.Store, *string, error) {
	if f.setPhotoErr != nil {
		return nil, nil, f.setPhotoErr
	}
	previous := f.storePhoto
	f.storePhoto = url
	store := f.stores[userID]
	store.PhotoURL = url
	return &store, previous, nil
}

type fakeImages struct {
	saved   []string
	deleted []string
}

func (f *fakeImages) Save(img *imageupload.Image) (string, error) {
	url := "/uploads/" + strings.Repeat("a", 31) + string(rune('0'+len(f.saved))) + "." + img.Ext
	f.saved = append(f.saved, url)
	return url, nil
}

func (f *fakeImages) Delete(path string) error {
	f.deleted = append(f.deleted, path)
	return nil
}

type fixture struct {
	app        *App
	store      *fakeStore
	owner      Actor
	penitip    Actor
	stranger   Actor
	storeID    uuid.UUID
	businessID uuid.UUID
}

func newFixture() fixture {
	store := newFakeStore()
	ownerID, storeID, businessID := uuid.New(), uuid.New(), uuid.New()
	store.stores[ownerID] = domain.Store{ID: storeID, OwnerUserID: ownerID, Name: "Warung Bu Siti", IsActive: true}
	return fixture{
		app:        New(store, nil, nil, func() time.Time { return time.Unix(1_800_000_000, 0) }),
		store:      store,
		owner:      Actor{UserID: ownerID, AccountType: identity.AccountTypePemilikToko},
		penitip:    Actor{UserID: uuid.New(), AccountType: identity.AccountTypePenitip, BusinessID: businessID},
		stranger:   Actor{UserID: uuid.New(), AccountType: identity.AccountTypePenitip, BusinessID: uuid.New()},
		storeID:    storeID,
		businessID: businessID,
	}
}

func appErrCode(t *testing.T, err error) string {
	t.Helper()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected apperror, got %v", err)
	}
	return appErr.Code
}

func TestStoreOwnerOfferFlow(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	offer, err := f.app.CreateProposal(ctx, f.owner, CreateProposalInput{BusinessID: f.businessID, Message: "  Rak depan kasir  "})
	if err != nil {
		t.Fatal(err)
	}
	if offer.Direction != domain.ProposalStoreToPenitip || offer.StoreID != f.storeID || offer.BusinessID != f.businessID {
		t.Fatalf("unexpected offer: %+v", offer)
	}
	if offer.Message == nil || *offer.Message != "Rak depan kasir" {
		t.Fatalf("message should be trimmed, got %v", offer.Message)
	}

	// Pengirim tidak boleh menyetujui tawarannya sendiri.
	if _, err := f.app.RespondProposal(ctx, f.owner, offer.ID, true, nil); appErrCode(t, err) != "ONLY_RECIPIENT_CAN_RESPOND" {
		t.Fatalf("sender accepting own offer: %v", err)
	}
	// Penitip lain tidak boleh melihat/menanggapi.
	if _, err := f.app.RespondProposal(ctx, f.stranger, offer.ID, true, nil); appErrCode(t, err) != "NOT_FOUND" {
		t.Fatalf("stranger: %v", err)
	}
	// Penerima tidak boleh membatalkan.
	if _, err := f.app.CancelProposal(ctx, f.penitip, offer.ID); appErrCode(t, err) != "ONLY_SENDER_CAN_CANCEL" {
		t.Fatalf("recipient cancelling: %v", err)
	}

	accepted, err := f.app.RespondProposal(ctx, f.penitip, offer.ID, true, nil)
	if err != nil || accepted.Status != domain.ProposalAccepted {
		t.Fatalf("accept: %v %+v", err, accepted)
	}
	if _, err := f.app.RespondProposal(ctx, f.penitip, offer.ID, false, nil); appErrCode(t, err) != "PROPOSAL_NOT_PENDING" {
		t.Fatalf("responding twice: %v", err)
	}
}

func TestPenitipRequestValidation(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	productID := uuid.New()

	cases := map[string]CreateProposalInput{
		"missing store": {},
		"duplicate":     {StoreID: f.storeID, Items: []ProposalItemInput{{ProductID: productID, Qty: 1}, {ProductID: productID, Qty: 2}}},
		"qty zero":      {StoreID: f.storeID, Items: []ProposalItemInput{{ProductID: productID, Qty: 0}}},
		"long message":  {StoreID: f.storeID, Message: strings.Repeat("x", domain.MaxProposalMessage+1)},
	}
	for name, input := range cases {
		if _, err := f.app.CreateProposal(ctx, f.penitip, input); appErrCode(t, err) != apperror.CodeValidation {
			t.Errorf("%s: expected validation error, got %v", name, err)
		}
	}

	request, err := f.app.CreateProposal(ctx, f.penitip, CreateProposalInput{
		StoreID: f.storeID, Items: []ProposalItemInput{{ProductID: productID, Qty: 10}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.Direction != domain.ProposalPenitipToStore || request.BusinessID != f.businessID {
		t.Fatalf("unexpected request: %+v", request)
	}
	// Pemilik toko menolak pengajuan.
	rejected, err := f.app.RespondProposal(ctx, f.owner, request.ID, false, nil)
	if err != nil || rejected.Status != domain.ProposalRejected {
		t.Fatalf("reject: %v", err)
	}

	// Pemilik toko tidak boleh mengirim item produk.
	if _, err := f.app.CreateProposal(ctx, f.owner, CreateProposalInput{
		BusinessID: f.businessID, Items: []ProposalItemInput{{ProductID: productID, Qty: 1}},
	}); appErrCode(t, err) != apperror.CodeValidation {
		t.Fatalf("owner with items: %v", err)
	}
}

func TestPenitipCannotProposeToInactiveStore(t *testing.T) {
	f := newFixture()
	store := f.store.stores[f.owner.UserID]
	store.IsActive = false
	f.store.stores[f.owner.UserID] = store

	_, err := f.app.CreateProposal(context.Background(), f.penitip, CreateProposalInput{StoreID: f.storeID})
	if appErrCode(t, err) != "STORE_INACTIVE" {
		t.Fatalf("expected STORE_INACTIVE, got %v", err)
	}
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestStorePhotoReplaceDeletesPrevious(t *testing.T) {
	f := newFixture()
	images := &fakeImages{}
	f.app.WithImageStore(images, imageupload.DefaultMaxBytes)
	ctx := context.Background()

	first, err := f.app.SetMyStorePhoto(ctx, f.owner.UserID, bytes.NewReader(pngBytes(t)))
	if err != nil || first.PhotoURL == nil {
		t.Fatalf("first upload: %v", err)
	}
	if _, err := f.app.SetMyStorePhoto(ctx, f.owner.UserID, bytes.NewReader(pngBytes(t))); err != nil {
		t.Fatal(err)
	}
	if len(images.deleted) != 1 || images.deleted[0] != *first.PhotoURL {
		t.Fatalf("previous photo should be deleted, deleted=%v", images.deleted)
	}

	// Tipe salah ditolak dengan error validasi field "file" dan tidak menyimpan apa pun.
	saved := len(images.saved)
	_, err = f.app.SetMyStorePhoto(ctx, f.owner.UserID, strings.NewReader("not an image"))
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Fields["file"] == "" || appErr.FieldCodes()["file"] != apperror.FieldInvalidChoice {
		t.Fatalf("expected file validation error, got %v", err)
	}
	if len(images.saved) != saved {
		t.Fatal("invalid upload must not be saved")
	}

	// Jika update database gagal, file baru dibersihkan.
	f.store.setPhotoErr = errors.New("db down")
	_, _ = f.app.SetMyStorePhoto(ctx, f.owner.UserID, bytes.NewReader(pngBytes(t)))
	last := images.saved[len(images.saved)-1]
	if images.deleted[len(images.deleted)-1] != last {
		t.Fatal("new file should be deleted when the database update fails")
	}
}
