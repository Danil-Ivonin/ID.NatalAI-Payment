package payment

import (
	"github.com/google/uuid"
	"time"
)

type Payment struct {
	ID             uuid.UUID
	UserID         int64
	Amount         Money
	Description    string
	ProductCode    string
	Status         Status
	IdempotencyKey string
	PaidAt         *time.Time
	ExpiresAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (p *Payment) MarkWaitingForPayment() error {
	if p.Status != StatusCreated {
		return ErrInvalidStatusTransition
	}
	p.Status = StatusWaitingForPayment
	return nil
}

func (p *Payment) MarkSucceeded(paidAt time.Time) error {
	if p.Status != StatusWaitingForPayment {
		return ErrInvalidStatusTransition
	}
	p.Status = StatusSucceeded
	p.PaidAt = &paidAt
	return nil
}

func (p *Payment) MarkExpired() error {
	if p.Status != StatusWaitingForPayment {
		return ErrInvalidStatusTransition
	}
	p.Status = StatusExpired
	return nil
}
