package ports

import (
	"context"
	"encoding/json"
)

type EventPublisher interface {
	Publish(ctx context.Context, exchange string, routingKey string, body json.RawMessage) error
}
