package httpapi

import (
	"strconv"
	"strings"
	"time"

	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
	"github.com/titipku/titipku-backend/services/core-service/internal/usecase"
)

type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type businessResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	OwnerName string  `json:"owner_name"`
	WhatsApp  *string `json:"whatsapp,omitempty"`
	Address   *string `json:"address,omitempty"`
}

type authResponse struct {
	AccessToken string           `json:"access_token"`
	TokenType   string           `json:"token_type"`
	ExpiresAt   time.Time        `json:"expires_at"`
	User        userResponse     `json:"user"`
	Business    businessResponse `json:"business"`
}

type productResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	PhotoPath     *string   `json:"photo_path,omitempty"`
	ConsignPrice  int64     `json:"consign_price"`
	RetailPrice   int64     `json:"retail_price"`
	ShelfLifeDays int32     `json:"shelf_life_days"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type partnerResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	OwnerName  string    `json:"owner_name"`
	WhatsApp   string    `json:"whatsapp"`
	Address    *string   `json:"address,omitempty"`
	IsActive   bool      `json:"is_active"`
	Receivable int64     `json:"receivable"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type visitItemResponse struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	PrevStock   int64  `json:"prev_stock"`
	Remaining   int64  `json:"remaining"`
	Returned    int64  `json:"returned"`
	NewDrop     int64  `json:"new_drop"`
	Sold        int64  `json:"sold"`
	StockAfter  int64  `json:"stock_after"`
	UnitPrice   int64  `json:"unit_price"`
	Subtotal    int64  `json:"subtotal"`
}

type visitResponse struct {
	ID               string              `json:"id"`
	Number           string              `json:"number"`
	BusinessName     string              `json:"business_name"`
	PartnerID        string              `json:"partner_id"`
	PartnerName      string              `json:"partner_name"`
	PartnerOwner     string              `json:"partner_owner_name"`
	PartnerWhatsApp  string              `json:"partner_whatsapp"`
	VisitedAt        time.Time           `json:"visited_at"`
	ReceivedBy       string              `json:"received_by"`
	Total            int64               `json:"total"`
	PublicToken      string              `json:"public_token,omitempty"`
	PublicInvoiceURL string              `json:"public_invoice_url,omitempty"`
	WhatsAppURL      string              `json:"whatsapp_url,omitempty"`
	Items            []visitItemResponse `json:"items"`
	CreatedAt        time.Time           `json:"created_at"`
}

type invoiceResponse struct {
	ID               string              `json:"id"`
	PartnerID        string              `json:"partner_id"`
	PartnerName      string              `json:"partner_name"`
	PartnerWhatsApp  string              `json:"partner_whatsapp"`
	VisitID          string              `json:"visit_id"`
	VisitNumber      string              `json:"visit_number"`
	VisitDate        time.Time           `json:"visit_date"`
	ReceivedBy       string              `json:"received_by"`
	Total            int64               `json:"total"`
	Paid             int64               `json:"paid"`
	Remaining        int64               `json:"remaining"`
	Status           string              `json:"status"`
	IssuedAt         time.Time           `json:"issued_at"`
	PublicInvoiceURL string              `json:"public_invoice_url,omitempty"`
	Items            []visitItemResponse `json:"items,omitempty"`
}

type paymentResponse struct {
	ID          string    `json:"id"`
	PartnerID   string    `json:"partner_id"`
	PartnerName string    `json:"partner_name"`
	Amount      int64     `json:"amount"`
	Method      string    `json:"method"`
	PaidAt      time.Time `json:"paid_at"`
	IsVoid      bool      `json:"is_void"`
	CreatedAt   time.Time `json:"created_at"`
}

type recentVisitResponse struct {
	ID          string    `json:"id"`
	Number      string    `json:"number"`
	PartnerID   string    `json:"partner_id"`
	PartnerName string    `json:"partner_name"`
	Total       int64     `json:"total"`
	Status      string    `json:"status"`
	VisitedAt   time.Time `json:"visited_at"`
}

type priorityPartnerResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	OwnerName  string     `json:"owner_name"`
	WhatsApp   string     `json:"whatsapp"`
	Receivable int64      `json:"receivable"`
	LastVisit  *time.Time `json:"last_visit,omitempty"`
}

type dashboardResponse struct {
	Date             string                    `json:"date"`
	Receivable       int64                     `json:"receivable"`
	PaidToday        int64                     `json:"paid_today"`
	VisitsToday      int64                     `json:"visits_today"`
	UnpaidInvoices   int64                     `json:"unpaid_invoices"`
	ActivePartners   int64                     `json:"active_partners"`
	RecentVisits     []recentVisitResponse     `json:"recent_visits"`
	PriorityPartners []priorityPartnerResponse `json:"priority_partners"`
}

type publicVisitItemResponse struct {
	ProductName string `json:"product_name"`
	Remaining   int64  `json:"remaining"`
	Returned    int64  `json:"returned"`
	NewDrop     int64  `json:"new_drop"`
	Sold        int64  `json:"sold"`
	StockAfter  int64  `json:"stock_after"`
	UnitPrice   int64  `json:"unit_price"`
	Subtotal    int64  `json:"subtotal"`
}

type publicInvoiceResponse struct {
	Business struct {
		Name      string  `json:"name"`
		OwnerName string  `json:"owner_name"`
		WhatsApp  *string `json:"whatsapp,omitempty"`
		Address   *string `json:"address,omitempty"`
	} `json:"business"`
	Partner struct {
		Name      string `json:"name"`
		OwnerName string `json:"owner_name"`
		WhatsApp  string `json:"whatsapp"`
	} `json:"partner"`
	VisitNumber string                    `json:"visit_number"`
	VisitDate   time.Time                 `json:"visit_date"`
	ReceivedBy  string                    `json:"received_by"`
	Items       []publicVisitItemResponse `json:"items"`
	Total       int64                     `json:"total"`
	Paid        int64                     `json:"paid"`
	Remaining   int64                     `json:"remaining"`
	Status      string                    `json:"status"`
}

func newAuthResponse(result *usecase.AuthResult) authResponse {
	return authResponse{
		AccessToken: result.Token,
		TokenType:   "Bearer",
		ExpiresAt:   result.ExpiresAt,
		User:        newUserResponse(result.Account.User),
		Business:    newBusinessResponse(result.Account.Business),
	}
}

func newUserResponse(user domain.User) userResponse {
	return userResponse{
		ID:        user.ID.String(),
		Email:     user.Email,
		FullName:  user.FullName,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	}
}

func newBusinessResponse(business domain.Business) businessResponse {
	return businessResponse{
		ID:        business.ID.String(),
		Name:      business.Name,
		OwnerName: business.OwnerName,
		WhatsApp:  business.WhatsApp,
		Address:   business.Address,
	}
}

func newProductResponse(product domain.Product) productResponse {
	return productResponse{
		ID:            product.ID.String(),
		Name:          product.Name,
		PhotoPath:     product.PhotoPath,
		ConsignPrice:  product.ConsignPrice,
		RetailPrice:   product.RetailPrice,
		ShelfLifeDays: product.ShelfLifeDays,
		IsActive:      product.IsActive,
		CreatedAt:     product.CreatedAt,
		UpdatedAt:     product.UpdatedAt,
	}
}

func newProductResponses(products []domain.Product) []productResponse {
	responses := make([]productResponse, 0, len(products))
	for _, product := range products {
		responses = append(responses, newProductResponse(product))
	}
	return responses
}

func newPartnerResponse(partner domain.Partner) partnerResponse {
	return partnerResponse{
		ID:         partner.ID.String(),
		Name:       partner.Name,
		OwnerName:  partner.OwnerName,
		WhatsApp:   partner.WhatsApp,
		Address:    partner.Address,
		IsActive:   partner.IsActive,
		Receivable: partner.Receivable,
		CreatedAt:  partner.CreatedAt,
		UpdatedAt:  partner.UpdatedAt,
	}
}

func newPartnerResponses(partners []domain.Partner) []partnerResponse {
	responses := make([]partnerResponse, 0, len(partners))
	for _, partner := range partners {
		responses = append(responses, newPartnerResponse(partner))
	}
	return responses
}

func newVisitItemResponses(items []domain.VisitItem) []visitItemResponse {
	responses := make([]visitItemResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, visitItemResponse{
			ProductID:   item.ProductID.String(),
			ProductName: item.ProductName,
			PrevStock:   item.PrevStock,
			Remaining:   item.Remaining,
			Returned:    item.Returned,
			NewDrop:     item.NewDrop,
			Sold:        item.Sold,
			StockAfter:  item.StockAfter,
			UnitPrice:   item.UnitPrice,
			Subtotal:    item.Subtotal,
		})
	}
	return responses
}

func (h *Handler) newVisitResponse(visit domain.Visit) visitResponse {
	response := visitResponse{
		ID:              visit.ID.String(),
		Number:          visit.Number,
		BusinessName:    visit.BusinessName,
		PartnerID:       visit.PartnerID.String(),
		PartnerName:     visit.PartnerName,
		PartnerOwner:    visit.PartnerOwner,
		PartnerWhatsApp: visit.PartnerWhatsApp,
		VisitedAt:       visit.VisitedAt,
		ReceivedBy:      visit.ReceivedBy,
		Total:           visit.Total,
		Items:           newVisitItemResponses(visit.Items),
		CreatedAt:       visit.CreatedAt,
	}
	if visit.Total > 0 && visit.PublicToken != "" {
		response.PublicToken = visit.PublicToken
		response.PublicInvoiceURL = h.publicInvoiceURL(visit.PublicToken)
		response.WhatsAppURL = h.whatsappURL(
			visit.PartnerWhatsApp,
			visit.Number,
			visit.BusinessName,
			visit.Total,
			visit.PublicToken,
		)
	}
	return response
}

func (h *Handler) newVisitResponses(visits []domain.Visit) []visitResponse {
	responses := make([]visitResponse, 0, len(visits))
	for _, visit := range visits {
		responses = append(responses, h.newVisitResponse(visit))
	}
	return responses
}

func (h *Handler) newInvoiceResponse(invoice domain.Invoice, withItems bool) invoiceResponse {
	response := invoiceResponse{
		ID:               invoice.ID.String(),
		PartnerID:        invoice.PartnerID.String(),
		PartnerName:      invoice.PartnerName,
		PartnerWhatsApp:  invoice.PartnerWhatsApp,
		VisitID:          invoice.VisitID.String(),
		VisitNumber:      invoice.VisitNumber,
		VisitDate:        invoice.VisitDate,
		ReceivedBy:       invoice.ReceivedBy,
		Total:            invoice.Total,
		Paid:             invoice.Paid,
		Remaining:        invoice.Total - invoice.Paid,
		Status:           invoice.Status,
		IssuedAt:         invoice.IssuedAt,
		PublicInvoiceURL: h.publicInvoiceURL(invoice.PublicToken),
	}
	if withItems {
		response.Items = newVisitItemResponses(invoice.Items)
	}
	return response
}

func (h *Handler) newInvoiceResponses(invoices []domain.Invoice) []invoiceResponse {
	responses := make([]invoiceResponse, 0, len(invoices))
	for _, invoice := range invoices {
		responses = append(responses, h.newInvoiceResponse(invoice, false))
	}
	return responses
}

func newPaymentResponse(payment domain.Payment) paymentResponse {
	return paymentResponse{
		ID:          payment.ID.String(),
		PartnerID:   payment.PartnerID.String(),
		PartnerName: payment.PartnerName,
		Amount:      payment.Amount,
		Method:      payment.Method,
		PaidAt:      payment.PaidAt,
		IsVoid:      payment.IsVoid,
		CreatedAt:   payment.CreatedAt,
	}
}

func newDashboardResponse(dashboard domain.Dashboard) dashboardResponse {
	response := dashboardResponse{
		Date:             dashboard.Date,
		Receivable:       dashboard.Receivable,
		PaidToday:        dashboard.PaidToday,
		VisitsToday:      dashboard.VisitsToday,
		UnpaidInvoices:   dashboard.UnpaidInvoices,
		ActivePartners:   dashboard.ActivePartners,
		RecentVisits:     make([]recentVisitResponse, 0, len(dashboard.RecentVisits)),
		PriorityPartners: make([]priorityPartnerResponse, 0, len(dashboard.PriorityPartners)),
	}
	for _, visit := range dashboard.RecentVisits {
		response.RecentVisits = append(response.RecentVisits, recentVisitResponse{
			ID:          visit.ID.String(),
			Number:      visit.Number,
			PartnerID:   visit.PartnerID.String(),
			PartnerName: visit.PartnerName,
			Total:       visit.Total,
			Status:      visit.Status,
			VisitedAt:   visit.VisitedAt,
		})
	}
	for _, partner := range dashboard.PriorityPartners {
		response.PriorityPartners = append(response.PriorityPartners, priorityPartnerResponse{
			ID:         partner.ID.String(),
			Name:       partner.Name,
			OwnerName:  partner.OwnerName,
			WhatsApp:   partner.WhatsApp,
			Receivable: partner.Receivable,
			LastVisit:  partner.LastVisit,
		})
	}
	return response
}

func newPublicInvoiceResponse(invoice domain.Invoice) publicInvoiceResponse {
	items := make([]publicVisitItemResponse, 0, len(invoice.Items))
	for _, item := range invoice.Items {
		items = append(items, publicVisitItemResponse{
			ProductName: item.ProductName,
			Remaining:   item.Remaining,
			Returned:    item.Returned,
			NewDrop:     item.NewDrop,
			Sold:        item.Sold,
			StockAfter:  item.StockAfter,
			UnitPrice:   item.UnitPrice,
			Subtotal:    item.Subtotal,
		})
	}

	response := publicInvoiceResponse{
		VisitNumber: invoice.VisitNumber,
		VisitDate:   invoice.VisitDate,
		ReceivedBy:  invoice.ReceivedBy,
		Items:       items,
		Total:       invoice.Total,
		Paid:        invoice.Paid,
		Remaining:   invoice.Total - invoice.Paid,
		Status:      invoice.Status,
	}
	response.Business.Name = invoice.BusinessName
	response.Business.OwnerName = invoice.BusinessOwner
	response.Business.WhatsApp = invoice.BusinessWhatsApp
	response.Business.Address = invoice.BusinessAddress
	response.Partner.Name = invoice.PartnerName
	response.Partner.OwnerName = invoice.PartnerOwner
	response.Partner.WhatsApp = invoice.PartnerWhatsApp
	return response
}

func formatRupiah(amount int64) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}

	digits := strconv.FormatInt(amount, 10)
	var builder strings.Builder
	if negative {
		builder.WriteByte('-')
	}
	builder.WriteString("Rp")
	for index, digit := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			builder.WriteByte('.')
		}
		builder.WriteRune(digit)
	}
	return builder.String()
}
