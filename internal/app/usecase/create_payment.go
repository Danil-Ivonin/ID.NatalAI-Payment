package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports/repository"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/domain/payment"
	"github.com/google/uuid"
)

const robokassaProvider = "robokassa"

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
	ProductCode    string
	IdempotencyKey string
	ExpiresAt      *time.Time
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
	var response CreatePaymentResponse

	if err := u.txManager.WithinTx(ctx, func(ctx context.Context, tx ports.Tx) error {
		existingPayment, ok, err := u.paymentRepo.FindByIdempotencyKey(ctx, tx, req.IdempotencyKey)
		if err != nil {
			return fmt.Errorf("finding payment by idempotency key: %w", err)
		}
		if ok {
			invoice, err := u.providerInvoiceRepo.FindByPaymentID(ctx, tx, existingPayment.ID, robokassaProvider)
			if err != nil {
				return fmt.Errorf("finding provider invoice for existing payment: %w", err)
			}
			response = createPaymentResponse(existingPayment, invoice)
			return nil
		}

		money, err := payment.NewMoney(req.AmountMinor, req.Currency)
		if err != nil {
			return err
		}

		newPayment := payment.Payment{
			UserID:         req.UserID,
			Amount:         money,
			Description:    req.Description,
			ProductCode:    req.ProductCode,
			Status:         payment.StatusCreated,
			IdempotencyKey: req.IdempotencyKey,
			ExpiresAt:      req.ExpiresAt,
		}

		newPayment, err = u.paymentRepo.Create(ctx, tx, newPayment)
		if err != nil {
			return fmt.Errorf("creating payment: %w", err)
		}

		providerInvoiceID, err := u.providerInvoiceRepo.NextProviderInvoiceID(ctx, tx)
		if err != nil {
			return fmt.Errorf("getting next provider invoice id: %w", err)
		}

		paymentURL, err := u.paymentProvider.BuildPaymentURL(ports.BuildPaymentURLRequest{
			ProviderInvoiceID: providerInvoiceID,
			AmountMinor:       req.AmountMinor,
			Currency:          req.Currency,
			Description:       req.Description,
		})
		if err != nil {
			return fmt.Errorf("building robokassa payment url: %w", err)
		}

		invoice := repository.ProviderInvoice{
			PaymentID:         newPayment.ID,
			Provider:          robokassaProvider,
			ProviderInvoiceID: providerInvoiceID,
			PaymentURL:        paymentURL,
		}
		if err := u.providerInvoiceRepo.Create(ctx, tx, invoice); err != nil {
			return fmt.Errorf("creating provider invoice: %w", err)
		}

		fromStatus := newPayment.Status
		if err := newPayment.MarkWaitingForPayment(); err != nil {
			return err
		}
		if err := u.paymentRepo.Update(ctx, tx, newPayment); err != nil {
			return fmt.Errorf("marking payment waiting for payment: %w", err)
		}

		metadata, err := json.Marshal(map[string]any{
			"provider":            robokassaProvider,
			"provider_invoice_id": providerInvoiceID,
		})
		if err != nil {
			return fmt.Errorf("marshalling status history metadata: %w", err)
		}
		if err := u.statusHistoryRepo.Create(ctx, tx, repository.StatusHistory{
			PaymentID:  newPayment.ID,
			FromStatus: &fromStatus,
			ToStatus:   newPayment.Status,
			Reason:     "payment_url_created",
			Metadata:   metadata,
		}); err != nil {
			return fmt.Errorf("creating payment status history: %w", err)
		}

		response = createPaymentResponse(newPayment, invoice)
		return nil
	}); err != nil {
		return CreatePaymentResponse{}, err
	}

	return response, nil
}

func createPaymentResponse(p payment.Payment, invoice repository.ProviderInvoice) CreatePaymentResponse {
	return CreatePaymentResponse{
		PaymentID:         p.ID,
		Provider:          invoice.Provider,
		ProviderInvoiceID: invoice.ProviderInvoiceID,
		Status:            p.Status,
		PaymentURL:        invoice.PaymentURL,
	}
}
