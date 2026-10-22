package httpapi

import (
	"time"

	"github.com/titipku/titipku-backend/services/order-service/internal/domain"
)

type createOrderRequest struct {
	ItemName       string `json:"item_name"`
	Description    string `json:"description"`
	PickupAddress  string `json:"pickup_address"`
	DropoffAddress string `json:"dropoff_address"`
	ItemPrice      *int64 `json:"item_price"`
	ServiceFee     *int64 `json:"service_fee"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

type orderResponse struct {
	ID             string    `json:"id"`
	CustomerID     string    `json:"customer_id"`
	CourierID      *string   `json:"courier_id"`
	ItemName       string    `json:"item_name"`
	Description    string    `json:"description"`
	PickupAddress  string    `json:"pickup_address"`
	DropoffAddress string    `json:"dropoff_address"`
	ItemPrice      int64     `json:"item_price"`
	ServiceFee     int64     `json:"service_fee"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func newOrderResponse(order domain.Order) orderResponse {
	var courierID *string
	if order.CourierID != nil {
		value := order.CourierID.String()
		courierID = &value
	}

	return orderResponse{
		ID:             order.ID.String(),
		CustomerID:     order.CustomerID.String(),
		CourierID:      courierID,
		ItemName:       order.ItemName,
		Description:    order.Description,
		PickupAddress:  order.PickupAddress,
		DropoffAddress: order.DropoffAddress,
		ItemPrice:      order.ItemPrice,
		ServiceFee:     order.ServiceFee,
		Status:         string(order.Status),
		CreatedAt:      order.CreatedAt,
		UpdatedAt:      order.UpdatedAt,
	}
}

func newOrderResponses(orders []domain.Order) []orderResponse {
	responses := make([]orderResponse, 0, len(orders))
	for _, order := range orders {
		responses = append(responses, newOrderResponse(order))
	}
	return responses
}
