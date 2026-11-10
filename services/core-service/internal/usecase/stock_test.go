package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

// PartnerStock palsu: partner uuid.Nil dianggap tidak ada.
func (f *fakeStore) PartnerStock(_ context.Context, _, partnerID uuid.UUID) ([]domain.PartnerStockItem, error) {
	if partnerID == uuid.Nil {
		return nil, domain.ErrNotFound
	}
	return []domain.PartnerStockItem{{ProductID: uuid.New(), ProductName: "Keripik", IsActive: true, OnShelf: 4, EverConsigned: true}}, nil
}

func TestPartnerStock(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	items, err := f.app.PartnerStock(ctx, f.businessID, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].OnShelf != 4 {
		t.Fatalf("unexpected stock: %+v", items)
	}

	if _, err := f.app.PartnerStock(ctx, f.businessID, uuid.Nil); appErrCode(t, err) != "PARTNER_NOT_FOUND" {
		t.Fatalf("want PARTNER_NOT_FOUND, got %v", err)
	}
}
