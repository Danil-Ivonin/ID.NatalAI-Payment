package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports/repository"
)

type HandleProviderResultUsecase struct {
	txManager         ports.TxManager
	paymentProvider   ports.PaymentProvider
	providerEventRepo repository.ProviderEventRepository
	confirmPayment    *ConfirmPaymentUsecase
}

type HandleProviderResultRequest struct {
	Provider      string
	Values        map[string]string
	RawPayload    json.RawMessage
	ReceivedAt    time.Time
	SuccessReason string
	FailureReason string
}

type HandleProviderResultResponse struct {
	Provider          string
	ProviderInvoiceID int64
	SignatureValid    bool
	AckBody           string
}

func NewHandleProviderResultUsecase(
	txManager ports.TxManager,
	paymentProvider ports.PaymentProvider,
	providerEventRepo repository.ProviderEventRepository,
	confirmPayment *ConfirmPaymentUsecase,
) *HandleProviderResultUsecase {
	return &HandleProviderResultUsecase{
		txManager:         txManager,
		paymentProvider:   paymentProvider,
		providerEventRepo: providerEventRepo,
		confirmPayment:    confirmPayment,
	}
}

func (u *HandleProviderResultUsecase) Execute(
	ctx context.Context,
	req HandleProviderResultRequest,
) (HandleProviderResultResponse, error) {
	return HandleProviderResultResponse{}, ErrNotImplemented
}
