package payment

import (
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestPayment_MarkWaitingForPayment(t *testing.T) {
	t.Parallel()

	p := Payment{
		ID:             uuid.New(),
		UserID:         123,
		Amount:         Money{AmountMinor: 29900, Currency: "RUB"},
		Description:    "Покупка 1000 coins",
		ProductCode:    "coins_1000",
		Status:         StatusCreated,
		IdempotencyKey: "tg-123-coins-1000-001",
	}

	if err := p.MarkWaitingForPayment(); err != nil {
		t.Fatalf("MarkWaitingForPayment failed with %v", err)
	}

	if p.Status != StatusWaitingForPayment {
		t.Fatalf("Payment status %s, want %s", p.Status, StatusWaitingForPayment)
	}
}

func TestPayment_MarkSucceeded(t *testing.T) {
	t.Parallel()

	paidAt := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	p := Payment{
		ID:     uuid.New(),
		Status: StatusWaitingForPayment,
		Amount: Money{AmountMinor: 29900, Currency: "RUB"},
	}

	if err := p.MarkSucceeded(paidAt); err != nil {
		t.Fatalf("MarkSucceeded() unexpected error: %v", err)
	}
	if p.Status != StatusSucceeded {
		t.Fatalf("status = %s, want %s", p.Status, StatusSucceeded)
	}
	if p.PaidAt == nil || !p.PaidAt.Equal(paidAt) {
		t.Fatalf("paid_at = %v, want %v", p.PaidAt, paidAt)
	}
}

func TestPayment_MarkSucceededRejectsTerminalStatus(t *testing.T) {
	t.Parallel()

	p := Payment{ID: uuid.New(), Status: StatusSucceeded}

	if err := p.MarkSucceeded(time.Now()); err != ErrInvalidStatusTransition {
		t.Fatalf("MarkSucceeded() error = %v, want %v", err, ErrInvalidStatusTransition)
	}
}
