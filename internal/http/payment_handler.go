package http

import (
	"context"
	"net/http"
	"time"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/usecase"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/domain/payment"
	"github.com/google/uuid"
)

type CreatePaymentExecutor interface {
	Execute(context.Context, usecase.CreatePaymentRequest) (usecase.CreatePaymentResponse, error)
}

type GetPaymentExecutor interface {
	Execute(context.Context, usecase.GetPaymentRequest) (usecase.GetPaymentResponse, error)
}

type PaymentHandler struct {
	createPayment CreatePaymentExecutor
	getPayment    GetPaymentExecutor
}

type CreatePaymentRequest struct {
	UserID         int64      `json:"user_id"`
	AmountMinor    int64      `json:"amount_minor"`
	Currency       string     `json:"currency"`
	Description    string     `json:"description"`
	ProductCode    string     `json:"product_code"`
	IdempotencyKey string     `json:"idempotency_key"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

type PaymentResponse struct {
	PaymentID         uuid.UUID      `json:"payment_id"`
	UserID            int64          `json:"user_id,omitempty"`
	AmountMinor       int64          `json:"amount_minor,omitempty"`
	Currency          string         `json:"currency,omitempty"`
	Description       string         `json:"description,omitempty"`
	ProductCode       string         `json:"product_code,omitempty"`
	Provider          string         `json:"provider"`
	ProviderInvoiceID int64          `json:"provider_invoice_id"`
	Status            payment.Status `json:"status"`
	PaymentURL        string         `json:"payment_url"`
}

func NewPaymentHandler(
	createPayment CreatePaymentExecutor,
	getPayment GetPaymentExecutor,
) *PaymentHandler {
	return &PaymentHandler{
		createPayment: createPayment,
		getPayment:    getPayment,
	}
}

func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}

func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}
