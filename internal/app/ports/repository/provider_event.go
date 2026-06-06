package repository

import (
	"context"
	"encoding/json"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/google/uuid"
	"time"
)

type ProviderEvent struct {
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

type ProviderEventRepository interface {
	Create(ctx context.Context, tx ports.Tx, event ProviderEvent) error
	MarkProcessed(ctx context.Context, tx ports.Tx, provider string, payloadHash string, signatureValid bool) error
}
