package payment

import (
	"github.com/google/uuid"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusPaid      Status = "paid"
	StatusCancelled Status = "cancelled"
	StatusExpired   Status = "expired"
)

type Order struct {
	ID             uuid.UUID
	UserID         string
	GenerationID   uuid.UUID
	Status         Status
	AmountKopecks  int64
	Currency       string
	InvID          int64
	PaymentURL     string
	IdempotencyKey string
	PaidAt         *time.Time
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (o *Order) MarkPaid(paidTime time.Time) error {
	if o.Status != StatusPending {
		return ErrPaymentStatus
	}
	o.PaidAt = &paidTime
	o.Status = StatusPaid
	return nil
}
