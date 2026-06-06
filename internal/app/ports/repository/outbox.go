package repository

import (
	"context"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/domain/outbox"
	"github.com/google/uuid"
	"time"
)

type OutboxRepository interface {
	Create(ctx context.Context, tx ports.Tx, event outbox.Event) error
	LockPending(ctx context.Context, tx ports.Tx, workerID string, limit int) ([]outbox.Event, error)
	MarkPublished(ctx context.Context, tx ports.Tx, id uuid.UUID, publishedAt time.Time) error
	MarkFailed(ctx context.Context, tx ports.Tx, id uuid.UUID, attempts int, publishAfter time.Time, lastError string) error
}
