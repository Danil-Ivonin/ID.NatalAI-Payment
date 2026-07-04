package usecase

import (
	"context"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports/repository"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/domain/payment"
	"github.com/google/uuid"
)

type GetPaymentUsecase struct {
	txManager           ports.TxManager
	paymentRepo         repository.PaymentRepository
	providerInvoiceRepo repository.ProviderInvoiceRepository
}

type GetPaymentRequest struct {
	PaymentID uuid.UUID
}

type GetPaymentResponse struct {
	PaymentID         uuid.UUID
	UserID            int64
	AmountMinor       int64
	Currency          string
	Description       string
	ProductCode       string
	Status            payment.Status
	Provider          string
	ProviderInvoiceID int64
	PaymentURL        string
}

func NewGetPaymentUsecase(
	txManager ports.TxManager,
	paymentRepo repository.PaymentRepository,
	providerInvoiceRepo repository.ProviderInvoiceRepository,
) *GetPaymentUsecase {
	return &GetPaymentUsecase{
		txManager:           txManager,
		paymentRepo:         paymentRepo,
		providerInvoiceRepo: providerInvoiceRepo,
	}
}

func (u *GetPaymentUsecase) Execute(ctx context.Context, req GetPaymentRequest) (GetPaymentResponse, error) {
	return GetPaymentResponse{}, ErrNotImplemented
}
