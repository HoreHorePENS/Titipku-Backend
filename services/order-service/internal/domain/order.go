package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrOrderNotFound     = errors.New("order not found")
	ErrInvalidOrder      = errors.New("invalid order")
	ErrInvalidStatus     = errors.New("invalid order status")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrOrderNotAvailable = errors.New("order is not available")
)

const (
	MinItemNameLength    = 3
	MaxItemNameLength    = 120
	MaxDescriptionLength = 1000
	MinAddressLength     = 5
	MaxAddressLength     = 255
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusAccepted  Status = "accepted"
	StatusPickedUp  Status = "picked_up"
	StatusDelivered Status = "delivered"
	StatusCancelled Status = "cancelled"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusAccepted, StatusPickedUp, StatusDelivered, StatusCancelled:
		return true
	default:
		return false
	}
}

func CanTransition(from, to Status) bool {
	switch from {
	case StatusPending:
		return to == StatusAccepted || to == StatusCancelled
	case StatusAccepted:
		return to == StatusPickedUp || to == StatusCancelled
	case StatusPickedUp:
		return to == StatusDelivered
	default:
		return false
	}
}

type NewOrderInput struct {
	ItemName       string
	Description    string
	PickupAddress  string
	DropoffAddress string
	ItemPrice      int64
	ServiceFee     int64
}

func (i NewOrderInput) Validate() map[string]string {
	fields := make(map[string]string)

	itemName := strings.TrimSpace(i.ItemName)
	if length := utf8.RuneCountInString(itemName); length < MinItemNameLength || length > MaxItemNameLength {
		fields["item_name"] = "must be between 3 and 120 characters"
	}

	if utf8.RuneCountInString(strings.TrimSpace(i.Description)) > MaxDescriptionLength {
		fields["description"] = "must not exceed 1000 characters"
	}

	pickupAddress := strings.TrimSpace(i.PickupAddress)
	if length := utf8.RuneCountInString(pickupAddress); length < MinAddressLength || length > MaxAddressLength {
		fields["pickup_address"] = "must be between 5 and 255 characters"
	}

	dropoffAddress := strings.TrimSpace(i.DropoffAddress)
	if length := utf8.RuneCountInString(dropoffAddress); length < MinAddressLength || length > MaxAddressLength {
		fields["dropoff_address"] = "must be between 5 and 255 characters"
	}

	if i.ItemPrice < 0 {
		fields["item_price"] = "must not be negative"
	}
	if i.ServiceFee < 0 {
		fields["service_fee"] = "must not be negative"
	}

	return fields
}

type Order struct {
	ID             uuid.UUID
	CustomerID     uuid.UUID
	CourierID      *uuid.UUID
	ItemName       string
	Description    string
	PickupAddress  string
	DropoffAddress string
	ItemPrice      int64
	ServiceFee     int64
	Status         Status
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewOrder(id, customerID uuid.UUID, input NewOrderInput, now time.Time) (Order, error) {
	if len(input.Validate()) > 0 {
		return Order{}, ErrInvalidOrder
	}

	return Order{
		ID:             id,
		CustomerID:     customerID,
		ItemName:       strings.TrimSpace(input.ItemName),
		Description:    strings.TrimSpace(input.Description),
		PickupAddress:  strings.TrimSpace(input.PickupAddress),
		DropoffAddress: strings.TrimSpace(input.DropoffAddress),
		ItemPrice:      input.ItemPrice,
		ServiceFee:     input.ServiceFee,
		Status:         StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (o *Order) AssignCourier(courierID uuid.UUID, now time.Time) error {
	if courierID == uuid.Nil {
		return ErrInvalidOrder
	}
	if o.Status != StatusPending || o.CourierID != nil {
		return ErrOrderNotAvailable
	}

	o.CourierID = &courierID
	o.Status = StatusAccepted
	o.UpdatedAt = now
	return nil
}

func (o *Order) ChangeStatus(next Status, now time.Time) error {
	if !next.Valid() {
		return ErrInvalidStatus
	}
	if !CanTransition(o.Status, next) {
		return ErrInvalidTransition
	}

	o.Status = next
	o.UpdatedAt = now
	return nil
}
