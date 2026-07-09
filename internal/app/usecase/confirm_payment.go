package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports/repository"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/domain/payment"
	"github.com/google/uuid"
)

type ConfirmPaymentUsecase struct {
	txManager         ports.TxManager
	paymentRepo       repository.PaymentRepository
	statusHistoryRepo repository.StatusHistoryRepository
	outboxRepo        repository.OutboxRepository
}

type ConfirmPaymentRequest struct {
	PaymentID uuid.UUID
	PaidAt    time.Time
	Reason    string
	Metadata  json.RawMessage
}

type ConfirmPaymentResponse struct {
	PaymentID        uuid.UUID
	Status           payment.Status
	AlreadySucceeded bool
}

func NewConfirmPaymentUsecase(
	txManager ports.TxManager,
	paymentRepo repository.PaymentRepository,
	statusHistoryRepo repository.StatusHistoryRepository,
	outboxRepo repository.OutboxRepository,
) *ConfirmPaymentUsecase {
	return &ConfirmPaymentUsecase{
		txManager:         txManager,
		paymentRepo:       paymentRepo,
		statusHistoryRepo: statusHistoryRepo,
		outboxRepo:        outboxRepo,
	}
}

func (u *ConfirmPaymentUsecase) Execute(
	ctx context.Context,
	req ConfirmPaymentRequest,
) (ConfirmPaymentResponse, error) {
	return ConfirmPaymentResponse{}, ErrNotImplemented
}
