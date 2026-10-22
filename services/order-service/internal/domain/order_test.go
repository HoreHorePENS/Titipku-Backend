package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
		want bool
	}{
		{name: "pending to accepted", from: StatusPending, to: StatusAccepted, want: true},
		{name: "pending to cancelled", from: StatusPending, to: StatusCancelled, want: true},
		{name: "pending to delivered", from: StatusPending, to: StatusDelivered, want: false},
		{name: "accepted to picked up", from: StatusAccepted, to: StatusPickedUp, want: true},
		{name: "accepted to cancelled", from: StatusAccepted, to: StatusCancelled, want: true},
		{name: "picked up to delivered", from: StatusPickedUp, to: StatusDelivered, want: true},
		{name: "delivered to cancelled", from: StatusDelivered, to: StatusCancelled, want: false},
		{name: "cancelled to accepted", from: StatusCancelled, to: StatusAccepted, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanTransition(test.from, test.to); got != test.want {
				t.Fatalf("CanTransition(%s, %s) = %v, want %v", test.from, test.to, got, test.want)
			}
		})
	}
}

func TestAssignCourierOnlyWorksForPendingOrder(t *testing.T) {
	order := &Order{Status: StatusPending}
	courierID := uuid.New()

	if err := order.AssignCourier(courierID, time.Now()); err != nil {
		t.Fatalf("assign courier returned error: %v", err)
	}
	if order.Status != StatusAccepted {
		t.Fatalf("unexpected status: %s", order.Status)
	}
	if order.CourierID == nil || *order.CourierID != courierID {
		t.Fatal("courier was not assigned")
	}

	if err := order.AssignCourier(uuid.New(), time.Now()); !errors.Is(err, ErrOrderNotAvailable) {
		t.Fatalf("expected ErrOrderNotAvailable, got %v", err)
	}
}
