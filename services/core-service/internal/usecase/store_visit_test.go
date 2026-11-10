package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

// StoreVisitByID palsu: hanya nota dengan ID knownStoreVisit milik toko fixture.
var knownStoreVisit = uuid.MustParse("11111111-1111-1111-1111-111111111111")

func (f *fakeStore) StoreVisitByID(_ context.Context, storeID, visitID uuid.UUID) (*domain.Visit, error) {
	for _, store := range f.stores {
		if store.ID == storeID && visitID == knownStoreVisit {
			return &domain.Visit{ID: visitID, Number: "N-1", Total: 10000, Paid: 4000}, nil
		}
	}
	return nil, domain.ErrNotFound
}

func TestStoreVisitScopedToStoreOwner(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	visit, err := f.app.StoreVisit(ctx, f.owner, knownStoreVisit)
	if err != nil {
		t.Fatal(err)
	}
	if visit.Number != "N-1" || visit.Paid != 4000 {
		t.Fatalf("unexpected visit: %+v", visit)
	}

	if _, err := f.app.StoreVisit(ctx, f.owner, uuid.New()); appErrCode(t, err) != "VISIT_NOT_FOUND" {
		t.Fatalf("want VISIT_NOT_FOUND, got %v", err)
	}
	if _, err := f.app.StoreVisit(ctx, f.penitip, knownStoreVisit); appErrCode(t, err) != "STORE_OWNER_REQUIRED" {
		t.Fatalf("want STORE_OWNER_REQUIRED, got %v", err)
	}
}
