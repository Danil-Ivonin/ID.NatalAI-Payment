package repository

import (
	"context"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/domain/payment"
	"github.com/google/uuid"
	"time"
)

type PaymentRepository interface {
	Create(ctx context.Context, tx ports.Tx, payment payment.Payment) (payment.Payment, error)
	FindByID(ctx context.Context, tx ports.Tx, id uuid.UUID) (payment.Payment, error)
	FindByIdempotencyKey(ctx context.Context, tx ports.Tx, key string) (payment.Payment, bool, error)
	FindByProviderInvoiceIDForUpdate(ctx context.Context, tx ports.Tx, provider string, providerInvoiceID int64) (payment.Payment, error)
	Update(ctx context.Context, tx ports.Tx, payment payment.Payment) error
	ListWaitingForPayment(ctx context.Context, tx ports.Tx, olderThan time.Time, newerThan time.Time, limit int) ([]payment.Payment, error)
}
