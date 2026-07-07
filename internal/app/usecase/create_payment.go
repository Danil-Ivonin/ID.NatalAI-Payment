package usecase

import (
	"context"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports/repository"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/domain/payment"
	"github.com/google/uuid"
)

type CreatePaymentUsecase struct {
	txManager           ports.TxManager
	paymentProvider     ports.PaymentProvider
	paymentRepo         repository.PaymentRepository
	providerInvoiceRepo repository.ProviderInvoiceRepository
	statusHistoryRepo   repository.StatusHistoryRepository
}

type CreatePaymentRequest struct {
	UserID         int64
	AmountMinor    int64
	Currency       string
	Description    string
	IdempotencyKey string
}

type CreatePaymentResponse struct {
	PaymentID         uuid.UUID
	Provider          string
	ProviderInvoiceID int64
	Status            payment.Status
	PaymentURL        string
}

func NewCreatePaymentUsecase(
	txManager ports.TxManager,
	paymentProvider ports.PaymentProvider,
	paymentRepo repository.PaymentRepository,
	providerInvoiceRepo repository.ProviderInvoiceRepository,
	statusHistoryRepo repository.StatusHistoryRepository,
) *CreatePaymentUsecase {
	return &CreatePaymentUsecase{
		txManager:           txManager,
		paymentProvider:     paymentProvider,
		paymentRepo:         paymentRepo,
		providerInvoiceRepo: providerInvoiceRepo,
		statusHistoryRepo:   statusHistoryRepo,
	}
}

func (u *CreatePaymentUsecase) Execute(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error) {
	return CreatePaymentResponse{}, ErrNotImplemented
}
