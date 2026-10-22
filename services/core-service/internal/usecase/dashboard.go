package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

func (a *App) Dashboard(ctx context.Context, businessID uuid.UUID, day time.Time) (*domain.Dashboard, error) {
	if day.IsZero() {
		day = a.now()
	}

	dashboard, err := a.store.Dashboard(ctx, businessID, day)
	if err != nil {
		return nil, mapStoreError(err, "DASHBOARD_LOAD_FAILED", "unable to load dashboard")
	}
	if dashboard == nil {
		return nil, apperror.Internal("DASHBOARD_LOAD_FAILED", "dashboard is unavailable")
	}
	return dashboard, nil
}
