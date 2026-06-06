package repository

import (
	"context"
	"encoding/json"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/domain/payment"
	"github.com/google/uuid"
	"time"
)

type StatusHistory struct {
	ID         uuid.UUID
	PaymentID  uuid.UUID
	FromStatus *payment.Status
	ToStatus   payment.Status
	Reason     string
	Metadata   json.RawMessage
	CreatedAt  time.Time
}

type StatusHistoryRepository interface {
	Create(ctx context.Context, tx ports.Tx, history StatusHistory) error
}
