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
	Provider          string
	ProviderInvoiceID int64
	AmountMinor       int64
	Currency          string
	Values            map[string]string
	RawPayload        json.RawMessage
	ReceivedAt        time.Time
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
	response := HandleProviderResultResponse{}
	err := u.txManager.WithinTx(ctx, func(ctx context.Context, tx ports.Tx) error {
		event := repository.ProviderEvent{
			ID                uuid.UUID
			Provider          string
			ProviderInvoiceID int64
			EventType         string
			PayloadHash       string
			RawPayload        json.RawMessage
			SignatureValid    bool
			ReceivedAt        time.Time
			ProcessedAt       *time.Time
		}
		u.providerEventRepo.Create(ctx, tx, )

		u.paymentProvider.VerifyResultSignature()
		return nil
	}); if err != nil {
		return HandleProviderResultResponse{}, err
	}
	return response, nil
}
