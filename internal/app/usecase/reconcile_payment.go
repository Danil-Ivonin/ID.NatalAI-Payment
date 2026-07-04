package usecase

import (
	"context"
	"time"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports/repository"
)

type ReconcilePaymentUsecase struct {
	txManager           ports.TxManager
	paymentProvider     ports.PaymentProvider
	paymentRepo         repository.PaymentRepository
	providerInvoiceRepo repository.ProviderInvoiceRepository
	statusHistoryRepo   repository.StatusHistoryRepository
	confirmPayment      *ConfirmPaymentUsecase
}

type ReconcilePaymentRequest struct {
	OlderThan time.Time
	NewerThan time.Time
	Now       time.Time
	Limit     int
}

type ReconcilePaymentResponse struct {
	Checked   int
	Succeeded int
	Expired   int
	Pending   int
	Failed    int
}

func NewReconcilePaymentUsecase(
	txManager ports.TxManager,
	paymentProvider ports.PaymentProvider,
	paymentRepo repository.PaymentRepository,
	providerInvoiceRepo repository.ProviderInvoiceRepository,
	statusHistoryRepo repository.StatusHistoryRepository,
	confirmPayment *ConfirmPaymentUsecase,
) *ReconcilePaymentUsecase {
	return &ReconcilePaymentUsecase{
		txManager:           txManager,
		paymentProvider:     paymentProvider,
		paymentRepo:         paymentRepo,
		providerInvoiceRepo: providerInvoiceRepo,
		statusHistoryRepo:   statusHistoryRepo,
		confirmPayment:      confirmPayment,
	}
}

func (u *ReconcilePaymentUsecase) Execute(
	ctx context.Context,
	req ReconcilePaymentRequest,
) (ReconcilePaymentResponse, error) {
	return ReconcilePaymentResponse{}, ErrNotImplemented
}
