package repository

import (
	"context"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/google/uuid"
	"time"
)

type ProviderInvoice struct {
	ID                uuid.UUID
	PaymentID         uuid.UUID
	Provider          string
	ProviderInvoiceID int64
	PaymentURL        string
	ProviderStatus    *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ProviderInvoiceRepository interface {
	NextProviderInvoiceID(ctx context.Context, tx ports.Tx) (int64, error)
	Create(ctx context.Context, tx ports.Tx, invoice ProviderInvoice) error
	FindByPaymentID(ctx context.Context, tx ports.Tx, paymentID uuid.UUID, provider string) (ProviderInvoice, error)
}
