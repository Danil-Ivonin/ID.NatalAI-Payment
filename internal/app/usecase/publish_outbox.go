package usecase

import (
	"context"
	"time"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports/repository"
)

type PublishOutboxUsecase struct {
	txManager  ports.TxManager
	outboxRepo repository.OutboxRepository
	publisher  ports.EventPublisher
}

type PublishOutboxRequest struct {
	WorkerID          string
	BatchSize         int
	Exchange          string
	MaxAttempts       int
	RetryAfter        time.Duration
	LastErrorMaxBytes int
}

type PublishOutboxResponse struct {
	Locked    int
	Published int
	Failed    int
}

func NewPublishOutboxUsecase(
	txManager ports.TxManager,
	outboxRepo repository.OutboxRepository,
	publisher ports.EventPublisher,
) *PublishOutboxUsecase {
	return &PublishOutboxUsecase{
		txManager:  txManager,
		outboxRepo: outboxRepo,
		publisher:  publisher,
	}
}

func (u *PublishOutboxUsecase) Execute(
	ctx context.Context,
	req PublishOutboxRequest,
) (PublishOutboxResponse, error) {
	return PublishOutboxResponse{}, ErrNotImplemented
}
