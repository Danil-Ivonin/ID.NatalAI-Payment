# Payment Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Построить Go payment-service для Robokassa с идемпотентным созданием платежей, подтверждением через ResultURL, PostgreSQL transactional outbox, RabbitMQ publishing и reconciliation worker.

**Architecture:** Сохраняем Hexagonal Architecture из `ARCHITECTURE.md`: домен не зависит от Gin, pgx, RabbitMQ или Robokassa; application слой работает через ports; инфраструктура живет в adapters/http/config/logger. Composition root находится только в `cmd/*/main.go`, wiring выполняется manual constructor injection.

**Tech Stack:** Go 1.25.5, Gin, pgx/v5, PostgreSQL, RabbitMQ `amqp091-go`, Robokassa HTTP/signature integration, golang-migrate, `log/slog`, table-driven tests, integration tests через build tag `integration`.

---

## Текущий Контекст

Репозиторий уже содержит заготовки:

- `cmd/api/main.go` и `cmd/outbox-worker/main.go` существуют, но содержат только `package main`.
- `internal/domain/payment` содержит раннюю модель `Order`, статусы `pending/paid/...` и баг в `NewMoney`: `RUB` сейчас ошибочно считается неверной валютой.
- `internal/app/ports/*` и `internal/app/usecase/*` созданы как пустые пакеты.
- `migrations/000001_init.up.sql` уже создает базовые таблицы, но не хватает `outbox_events`, `payment_invoice_id_seq`, индексов payments и `reason` в истории статусов.
- `git status` показывает незакоммиченные изменения; при исполнении плана не откатывать существующие файлы без отдельного решения владельца.

## Файловая Карта

- Modify: `go.mod`, `go.sum` - добавить Gin, pgx, RabbitMQ client, golang-migrate driver dependencies, test helpers.
- Modify: `migrations/000001_init.up.sql`, `migrations/000001_init.down.sql` - довести схему до архитектуры, так как проект еще не выглядит запущенным в production.
- Move: `internal/domain/payment/order.go` -> `internal/domain/payment/entity.go` - заменить раннюю `Order` модель на `Payment` модель из архитектуры.
- Modify: `internal/domain/payment/status.go`, `internal/domain/payment/money.go`, `internal/domain/payment/errors.go`.
- Create: `internal/domain/payment/payment_test.go`, `internal/domain/payment/money_test.go`.
- Create: `internal/domain/outbox/event.go`, `internal/domain/outbox/event_test.go`.
- Modify: `internal/app/ports/*.go`, `internal/app/ports/repository/*.go` - определить интерфейсы application boundary.
- Modify: `internal/app/usecase/create_payment.go`, `confirm_payment.go`, `handle_result.go`, `publish_outbox.go`.
- Create: `internal/app/usecase/reconcile_payment.go`, `internal/app/usecase/*_test.go`.
- Create: `internal/adapters/postgres/*.go`, `internal/adapters/postgres/*_integration_test.go`.
- Create: `internal/adapters/robokassa/client.go`, `signer.go`, `url_builder.go`, `*_test.go`.
- Create: `internal/adapters/broker/rabbitmq.go`, `rabbitmq_test.go`.
- Create: `internal/http/router.go`, `payment_handler.go`, `robokassa_webhook_handler.go`, `middleware.go`, `*_test.go`.
- Modify: `internal/config/config.go`.
- Create: `internal/logger/logger.go`, `internal/observability/metrics.go`.
- Create: `cmd/reconciler/main.go`.
- Modify: `cmd/api/main.go`, `cmd/outbox-worker/main.go`.
- Create: `Makefile`, `.golangci.yml`, `docker-compose.yml`.
- Modify: `README.md`.

---

## Task 1: Dependencies And Tooling Baseline

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `Makefile`
- Create: `.golangci.yml`
- Create: `docker-compose.yml`

- [ ] **Step 1: Add runtime dependencies**

Run:

```powershell
go get github.com/gin-gonic/gin@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/rabbitmq/amqp091-go@latest
go get github.com/golang-migrate/migrate/v4@latest
go get github.com/prometheus/client_golang@latest
go get github.com/stretchr/testify@latest
go mod tidy
```

Expected: `go.mod` contains direct requirements for Gin, pgx/v5, amqp091-go, migrate, Prometheus client, testify.

- [ ] **Step 2: Create Makefile**

Create `Makefile`:

```makefile
.PHONY: test test-race test-integration lint fmt migrate-up migrate-down run-api run-outbox run-reconciler

test:
	go test ./...

test-race:
	go test -race ./...

test-integration:
	go test -tags=integration ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .

migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down 1

run-api:
	go run ./cmd/api

run-outbox:
	go run ./cmd/outbox-worker

run-reconciler:
	go run ./cmd/reconciler
```

- [ ] **Step 3: Create docker-compose for local backing services**

Create `docker-compose.yml`:

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: payment
      POSTGRES_PASSWORD: payment
      POSTGRES_DB: payment
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U payment -d payment"]
      interval: 5s
      timeout: 3s
      retries: 10

  rabbitmq:
    image: rabbitmq:3.13-management
    ports:
      - "5672:5672"
      - "15672:15672"
    healthcheck:
      test: ["CMD", "rabbitmq-diagnostics", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10
```

- [ ] **Step 4: Create minimal lint config**

Create `.golangci.yml`:

```yaml
version: "2"
run:
  timeout: 5m
linters:
  enable:
    - govet
    - staticcheck
    - errcheck
    - ineffassign
    - unused
    - bodyclose
    - sqlclosecheck
    - gosec
    - nilerr
    - revive
    - nolintlint
```

- [ ] **Step 5: Verify baseline**

Run:

```powershell
go test ./...
go test -race ./...
```

Expected: tests pass or package compilation fails only on intentionally empty stubs that are fixed in Task 2.

- [ ] **Step 6: Commit**

```powershell
git add go.mod go.sum Makefile .golangci.yml docker-compose.yml
git commit -m "chore: add payment service tooling baseline"
```

---

## Task 2: Domain Model

**Files:**
- Move: `internal/domain/payment/order.go` -> `internal/domain/payment/payment.go`
- Modify: `internal/domain/payment/status.go`
- Modify: `internal/domain/payment/money.go`
- Modify: `internal/domain/payment/errors.go`
- Create: `internal/domain/payment/payment_test.go`
- Create: `internal/domain/payment/money_test.go`
- Create: `internal/domain/outbox/event.go`
- Create: `internal/domain/outbox/event_test.go`

- [ ] **Step 1: Write failing tests for money validation**

Create `internal/domain/payment/money_test.go`:

```go
package payment

import "testing"

func TestNewMoney(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   int64
		currency string
		wantErr  error
	}{
		{name: "valid rub amount", amount: 29900, currency: "RUB", wantErr: nil},
		{name: "zero amount", amount: 0, currency: "RUB", wantErr: ErrInvalidAmount},
		{name: "negative amount", amount: -1, currency: "RUB", wantErr: ErrInvalidAmount},
		{name: "empty currency", amount: 100, currency: "", wantErr: ErrInvalidCurrency},
		{name: "unsupported currency", amount: 100, currency: "USD", wantErr: ErrInvalidCurrency},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewMoney(tt.amount, tt.currency)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("NewMoney() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewMoney() unexpected error: %v", err)
			}
			if got.AmountMinor != tt.amount || got.Currency != tt.currency {
				t.Fatalf("NewMoney() = %+v, want amount=%d currency=%s", got, tt.amount, tt.currency)
			}
		})
	}
}
```

- [ ] **Step 2: Write failing tests for status transitions**

Create `internal/domain/payment/payment_test.go`:

```go
package payment

import (
	"testing"
	"time"

	"github.com/google/uuid"
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
		t.Fatalf("MarkWaitingForPayment() unexpected error: %v", err)
	}
	if p.Status != StatusWaitingForPayment {
		t.Fatalf("status = %s, want %s", p.Status, StatusWaitingForPayment)
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
```

- [ ] **Step 3: Rename old order payment file**

Run:

```powershell
git mv internal/domain/payment/order.go internal/domain/payment/payment.go
```

Expected: old `Order` model will be replaced by the architecture-level `Payment` entity in the next step.

- [ ] **Step 4: Implement payment domain**

Replace domain files with:

```go
package payment

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusCreated           Status = "created"
	StatusWaitingForPayment Status = "waiting_for_payment"
	StatusSucceeded         Status = "succeeded"
	StatusFailed            Status = "failed"
	StatusExpired           Status = "expired"
	StatusCancelled         Status = "cancelled"
	StatusRefunded          Status = "refunded"
)

type Money struct {
	AmountMinor int64
	Currency    string
}

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

func NewMoney(amountMinor int64, currency string) (Money, error) {
	if amountMinor <= 0 {
		return Money{}, ErrInvalidAmount
	}
	if currency != "RUB" {
		return Money{}, ErrInvalidCurrency
	}
	return Money{AmountMinor: amountMinor, Currency: currency}, nil
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
```

Use `errors.go`:

```go
package payment

import "errors"

var (
	ErrInvalidAmount           = errors.New("invalid amount")
	ErrInvalidCurrency         = errors.New("invalid currency")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrPaymentNotFound         = errors.New("payment not found")
	ErrAmountMismatch          = errors.New("payment amount mismatch")
)
```

- [ ] **Step 5: Add outbox domain event**

Create `internal/domain/outbox/event.go`:

```go
package outbox

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusPublished  Status = "published"
	StatusFailed     Status = "failed"
)

type Event struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       json.RawMessage
	Status        Status
	Attempts      int
	PublishAfter  time.Time
	LockedAt      *time.Time
	LockedBy      *string
	PublishedAt   *time.Time
	LastError     *string
	CreatedAt     time.Time
}
```

- [ ] **Step 6: Verify domain**

Run:

```powershell
go test ./internal/domain/...
```

Expected: all domain tests pass.

- [ ] **Step 7: Commit**

```powershell
git add internal/domain
git commit -m "feat: define payment domain model"
```

---

## Task 3: Database Schema

**Files:**
- Modify: `migrations/000001_init.up.sql`
- Modify: `migrations/000001_init.down.sql`

- [ ] **Step 1: Extend up migration**

Update `migrations/000001_init.up.sql` to include:

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SEQUENCE payment_invoice_id_seq
    START WITH 10000000
    INCREMENT BY 1;

CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_created_at ON payments(created_at);
CREATE INDEX idx_payments_expires_at ON payments(expires_at);

ALTER TABLE payment_status_history
ADD COLUMN reason TEXT NOT NULL DEFAULT 'unknown';

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'published', 'failed')) DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    publish_after TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    locked_by TEXT,
    published_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT outbox_unique_business_event UNIQUE (aggregate_type, aggregate_id, event_type)
);

CREATE INDEX idx_outbox_pending
    ON outbox_events(status, publish_after, created_at)
    WHERE status IN ('pending', 'failed');
```

Place `CREATE EXTENSION` and sequence before table definitions, indexes after related tables, and `outbox_events` after status history.

- [ ] **Step 2: Extend down migration**

Update `migrations/000001_init.down.sql`:

```sql
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS payment_status_history;
DROP TABLE IF EXISTS payment_provider_events;
DROP TABLE IF EXISTS payment_provider_invoices;
DROP TABLE IF EXISTS payments;
DROP SEQUENCE IF EXISTS payment_invoice_id_seq;
```

- [ ] **Step 3: Verify migration roundtrip**

Run:

```powershell
docker compose up -d postgres
$env:DATABASE_URL='postgres://payment:payment@localhost:5432/payment?sslmode=disable'
migrate -path migrations -database $env:DATABASE_URL up
migrate -path migrations -database $env:DATABASE_URL down 1
migrate -path migrations -database $env:DATABASE_URL up
```

Expected: all commands complete without migration errors.

- [ ] **Step 4: Commit**

```powershell
git add migrations/000001_init.up.sql migrations/000001_init.down.sql
git commit -m "feat: complete payment database schema"
```

---

## Task 4: Application Ports

**Files:**
- Modify: `internal/app/ports/tx_manager.go`
- Modify: `internal/app/ports/payment_provider.go`
- Modify: `internal/app/ports/event_publisher.go`
- Modify: `internal/app/ports/repository/payment.go`
- Modify: `internal/app/ports/repository/provider_invoice.go`
- Modify: `internal/app/ports/repository/provider_event.go`
- Modify: `internal/app/ports/repository/status_history.go`
- Modify: `internal/app/ports/repository/outbox.go`

- [ ] **Step 1: Define transaction boundary**

Implement:

```go
package ports

import "context"

type Tx interface{}

type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error
}
```

- [ ] **Step 2: Define provider and publisher ports**

Implement `payment_provider.go`:

```go
package ports

import "context"

type ProviderPaymentStatus string

const (
	ProviderPaymentStatusPending   ProviderPaymentStatus = "pending"
	ProviderPaymentStatusSucceeded ProviderPaymentStatus = "succeeded"
	ProviderPaymentStatusFailed    ProviderPaymentStatus = "failed"
)

type BuildPaymentURLRequest struct {
	ProviderInvoiceID int64
	AmountMinor       int64
	Currency          string
	Description       string
}

type PaymentProvider interface {
	BuildPaymentURL(req BuildPaymentURLRequest) (string, error)
	VerifyResultSignature(values map[string]string) bool
	CheckPaymentStatus(ctx context.Context, providerInvoiceID int64) (ProviderPaymentStatus, error)
}
```

Implement `event_publisher.go`:

```go
package ports

import (
	"context"
	"encoding/json"
)

type EventPublisher interface {
	Publish(ctx context.Context, exchange string, routingKey string, body json.RawMessage) error
}
```

- [ ] **Step 3: Define repository ports**

Add repository DTOs and methods with `ctx context.Context` as the first argument:

```go
type PaymentRepository interface {
	Create(ctx context.Context, tx ports.Tx, payment payment.Payment) (payment.Payment, error)
	FindByID(ctx context.Context, tx ports.Tx, id uuid.UUID) (payment.Payment, error)
	FindByIdempotencyKey(ctx context.Context, tx ports.Tx, key string) (payment.Payment, bool, error)
	FindByProviderInvoiceIDForUpdate(ctx context.Context, tx ports.Tx, provider string, providerInvoiceID int64) (payment.Payment, error)
	Update(ctx context.Context, tx ports.Tx, payment payment.Payment) error
	ListWaitingForPayment(ctx context.Context, tx ports.Tx, olderThan time.Time, newerThan time.Time, limit int) ([]payment.Payment, error)
}
```

Define matching interfaces for provider invoices, provider events, status history and outbox:

```go
type ProviderInvoiceRepository interface {
	NextProviderInvoiceID(ctx context.Context, tx ports.Tx) (int64, error)
	Create(ctx context.Context, tx ports.Tx, invoice ProviderInvoice) error
	FindByPaymentID(ctx context.Context, tx ports.Tx, paymentID uuid.UUID, provider string) (ProviderInvoice, error)
}

type ProviderEventRepository interface {
	Create(ctx context.Context, tx ports.Tx, event ProviderEvent) error
	MarkProcessed(ctx context.Context, tx ports.Tx, provider string, payloadHash string, signatureValid bool) error
}

type StatusHistoryRepository interface {
	Create(ctx context.Context, tx ports.Tx, history StatusHistory) error
}

type OutboxRepository interface {
	Create(ctx context.Context, tx ports.Tx, event outbox.Event) error
	LockPending(ctx context.Context, tx ports.Tx, workerID string, limit int) ([]outbox.Event, error)
	MarkPublished(ctx context.Context, tx ports.Tx, id uuid.UUID, publishedAt time.Time) error
	MarkFailed(ctx context.Context, tx ports.Tx, id uuid.UUID, attempts int, publishAfter time.Time, lastError string) error
}
```

- [ ] **Step 4: Verify ports compile**

Run:

```powershell
go test ./internal/app/ports/...
```

Expected: package compiles.

- [ ] **Step 5: Commit**

```powershell
git add internal/app/ports
git commit -m "feat: define application ports"
```

---

## Task 5: CreatePayment Use Case

**Files:**
- Modify: `internal/app/usecase/create_payment.go`
- Create: `internal/app/usecase/create_payment_test.go`

- [ ] **Step 1: Write tests**

Cover these observable cases in `create_payment_test.go`:

- new request creates `payments`, `payment_provider_invoices`, status history `payment_url_created`, and returns Robokassa URL;
- same `idempotency_key` returns existing payment and invoice without new inserts;
- invalid amount rejects before repository calls;
- provider URL build failure rolls back transaction.

Use in-memory fakes for ports. The main success assertion:

```go
if got.Status != payment.StatusWaitingForPayment {
	t.Fatalf("status = %s, want %s", got.Status, payment.StatusWaitingForPayment)
}
if got.Provider != "robokassa" {
	t.Fatalf("provider = %s, want robokassa", got.Provider)
}
if got.ProviderInvoiceID != 10000001 {
	t.Fatalf("provider_invoice_id = %d, want 10000001", got.ProviderInvoiceID)
}
if got.PaymentURL == "" {
	t.Fatal("payment_url is empty")
}
```

- [ ] **Step 2: Implement request and response types**

Use:

```go
type CreatePaymentRequest struct {
	UserID         int64
	AmountMinor    int64
	Currency       string
	Description    string
	ProductCode    string
	IdempotencyKey string
	ExpiresAt      *time.Time
}

type CreatePaymentResponse struct {
	PaymentID         uuid.UUID
	Provider          string
	ProviderInvoiceID int64
	Status            payment.Status
	PaymentURL        string
}
```

- [ ] **Step 3: Implement use case**

`CreatePayment` must:

1. Validate `Money`, non-empty description, non-empty idempotency key.
2. Run all repository writes inside `TxManager.WithinTx`.
3. Query `FindByIdempotencyKey`; if found, return existing invoice.
4. Create payment with `StatusCreated`.
5. Generate `provider_invoice_id` via repository sequence.
6. Build URL through `PaymentProvider`.
7. Mark payment as `waiting_for_payment`.
8. Insert provider invoice.
9. Insert status history with reason `payment_url_created`.
10. Return response only after transaction function succeeds.

- [ ] **Step 4: Verify use case**

Run:

```powershell
go test ./internal/app/usecase -run TestCreatePayment -count=1
```

Expected: tests pass.

- [ ] **Step 5: Commit**

```powershell
git add internal/app/usecase/create_payment.go internal/app/usecase/create_payment_test.go
git commit -m "feat: implement create payment use case"
```

---

## Task 6: ConfirmPaymentSucceeded Use Case

**Files:**
- Modify: `internal/app/usecase/confirm_payment.go`
- Create: `internal/app/usecase/confirm_payment_test.go`

- [ ] **Step 1: Write tests**

Cover:

- `waiting_for_payment` becomes `succeeded`;
- `succeeded` input returns success without a second outbox event;
- amount mismatch returns `payment.ErrAmountMismatch`;
- outbox unique conflict is treated as idempotent success only when payment is already `succeeded`.

- [ ] **Step 2: Implement event payload contract**

Payload JSON must match architecture:

```json
{
  "event_id": "5f074414-22e8-4c4b-adc0-1d067c7c1c",
  "event_type": "PaymentSucceeded",
  "occurred_at": "2026-06-06T12:00:00Z",
  "payload": {
    "payment_id": "9b3dbd7e-30f3-49c7-b8b2-492508b7e6fa",
    "provider": "robokassa",
    "provider_invoice_id": 10000001,
    "user_id": 12345,
    "amount_minor": 29900,
    "currency": "RUB",
    "description": "Покупка 1000 coins",
    "product_code": "coins_1000"
  }
}
```

- [ ] **Step 3: Implement use case**

`ConfirmPaymentSucceeded` must run inside the caller's transaction, lock the payment before update, write status history with caller-provided reason, insert `PaymentSucceeded` outbox event with unique `(aggregate_type, aggregate_id, event_type)`.

- [ ] **Step 4: Verify**

Run:

```powershell
go test ./internal/app/usecase -run TestConfirmPaymentSucceeded -count=1
```

Expected: tests pass.

- [ ] **Step 5: Commit**

```powershell
git add internal/app/usecase/confirm_payment.go internal/app/usecase/confirm_payment_test.go
git commit -m "feat: confirm successful payments transactionally"
```

---

## Task 7: HandleRobokassaResult Use Case

**Files:**
- Modify: `internal/app/usecase/handle_result.go`
- Create: `internal/app/usecase/handle_result_test.go`

- [ ] **Step 1: Write tests**

Cover:

- valid callback persists provider event, confirms payment, commits, returns `OK10000001`;
- invalid signature persists provider event with `signature_valid=false`, does not confirm payment, returns an invalid signature error;
- duplicate callback for already succeeded payment returns `OK10000001`;
- amount mismatch does not create outbox event;
- response string is produced after transaction succeeds, not from inside the transaction closure.

- [ ] **Step 2: Implement request type**

Use:

```go
type RobokassaResultRequest struct {
	OutSum            string
	InvID             int64
	SignatureValue    string
	RawValues         map[string]string
	RawPayload        map[string]any
	ReceivedAt        time.Time
	ExpectedCurrency  string
	ExpectedAmountRaw int64
}
```

- [ ] **Step 3: Implement payload hash**

Use SHA-256 over canonical JSON of raw payload:

```go
func payloadHash(raw map[string]any) (string, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("marshal provider payload: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
```

- [ ] **Step 4: Implement use case**

Processing order:

1. Compute payload hash.
2. Start transaction.
3. Save provider event with `event_type='robokassa_result'`.
4. Verify signature.
5. If invalid, mark processed with invalid signature and return domain error after commit/rollback semantics are explicit in test.
6. Lock payment by provider invoice id.
7. Parse and compare amount in kopecks.
8. Call `ConfirmPaymentSucceeded` with reason `robokassa_result_url`.
9. Mark provider event processed with `signature_valid=true`.
10. Return `OK{InvId}` only after `WithinTx` returns nil.

- [ ] **Step 5: Verify**

Run:

```powershell
go test ./internal/app/usecase -run TestHandleRobokassaResult -count=1
```

Expected: tests pass.

- [ ] **Step 6: Commit**

```powershell
git add internal/app/usecase/handle_result.go internal/app/usecase/handle_result_test.go
git commit -m "feat: handle robokassa result callbacks"
```

---

## Task 8: PostgreSQL Adapters

**Files:**
- Create: `internal/adapters/postgres/db.go`
- Create: `internal/adapters/postgres/tx_manager.go`
- Create: `internal/adapters/postgres/payment_repository.go`
- Create: `internal/adapters/postgres/provider_invoice_repository.go`
- Create: `internal/adapters/postgres/provider_event_repository.go`
- Create: `internal/adapters/postgres/status_history_repository.go`
- Create: `internal/adapters/postgres/outbox_repository.go`
- Create: `internal/adapters/postgres/*_integration_test.go`

- [ ] **Step 1: Implement pgx pool factory**

`NewPool(ctx, databaseURL string)` must call `pgxpool.ParseConfig`, configure pool limits from config, `Ping(ctx)`, and return `*pgxpool.Pool`.

- [ ] **Step 2: Implement TxManager**

Use `pgx.Tx` as concrete transaction behind `ports.Tx`. `WithinTx` must commit on nil error, rollback on function error, and join rollback error with function error using `errors.Join`.

- [ ] **Step 3: Implement repositories with parameterized SQL**

Required SQL guarantees:

```sql
SELECT p.*
FROM payments p
JOIN payment_provider_invoices pi ON pi.payment_id = p.id
WHERE pi.provider = $1 AND pi.provider_invoice_id = $2
FOR UPDATE;
```

```sql
SELECT *
FROM outbox_events
WHERE status IN ('pending', 'failed')
  AND publish_after <= now()
ORDER BY created_at
LIMIT $1
FOR UPDATE SKIP LOCKED;
```

- [ ] **Step 4: Add integration tests**

Each integration test file starts with:

```go
//go:build integration
```

Tests use `TEST_DATABASE_URL`; if empty, call `t.Skip("TEST_DATABASE_URL is not set")`. Cover:

- idempotency unique constraint;
- row lock path for provider invoice lookup;
- outbox unique business event;
- outbox `FOR UPDATE SKIP LOCKED` batch lock;
- migration schema roundtrip.

- [ ] **Step 5: Verify**

Run:

```powershell
go test ./internal/adapters/postgres
$env:TEST_DATABASE_URL='postgres://payment:payment@localhost:5432/payment?sslmode=disable'
go test -tags=integration ./internal/adapters/postgres -count=1
```

Expected: unit compile passes; integration tests pass with local PostgreSQL.

- [ ] **Step 6: Commit**

```powershell
git add internal/adapters/postgres
git commit -m "feat: add postgres repositories"
```

---

## Task 9: Robokassa Adapter

**Files:**
- Create: `internal/adapters/robokassa/signer.go`
- Create: `internal/adapters/robokassa/url_builder.go`
- Create: `internal/adapters/robokassa/client.go`
- Create: `internal/adapters/robokassa/*_test.go`

- [ ] **Step 1: Write signer tests**

Use deterministic cases for ResultURL:

```go
values := map[string]string{
	"OutSum":         "299.00",
	"InvId":          "10000001",
	"SignatureValue": "DE98F9AEE063E4BC6CB7D8E572B764D0",
}
```

Assert signature comparison is case-insensitive and uses Robokassa password2 value `password2` for ResultURL verification.

- [ ] **Step 2: Implement signer**

Implement MD5 signature creation exactly as Robokassa requires for selected mode:

```go
func resultSignature(outSum string, invID int64, password2 string) string {
	raw := fmt.Sprintf("%s:%d:%s", outSum, invID, password2)
	sum := md5.Sum([]byte(raw))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
```

Use constant-time compare after normalizing hex strings:

```go
return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
```

- [ ] **Step 3: Implement URL builder**

Build `https://auth.robokassa.ru/Merchant/Index.aspx` with `MerchantLogin`, `OutSum`, `InvId`, `Description`, `SignatureValue`, `IsTest` and optional product metadata. Use `net/url.Values`, never string concatenation for query parameters.

- [ ] **Step 4: Implement status client**

`CheckPaymentStatus(ctx, providerInvoiceID)` must use an HTTP client with timeout, `http.NewRequestWithContext`, parse Robokassa response, and map provider result to `ProviderPaymentStatus`.

- [ ] **Step 5: Verify**

Run:

```powershell
go test ./internal/adapters/robokassa -count=1
```

Expected: tests pass.

- [ ] **Step 6: Commit**

```powershell
git add internal/adapters/robokassa
git commit -m "feat: add robokassa adapter"
```

---

## Task 10: HTTP API

**Files:**
- Create: `internal/http/router.go`
- Create: `internal/http/payment_handler.go`
- Create: `internal/http/robokassa_webhook_handler.go`
- Create: `internal/http/middleware.go`
- Create: `internal/http/*_test.go`

- [ ] **Step 1: Write handler tests**

Use `httptest` and fake use cases. Cover:

- `POST /v1/payments` returns `201` and JSON response;
- duplicate create returns existing response with `200`;
- validation error returns `400`;
- `POST /v1/webhooks/robokassa/result` returns plain text `OK10000001`;
- `GET /v1/webhooks/robokassa/success` does not call confirmation use case;
- `GET /v1/webhooks/robokassa/fail` does not mark payment failed.

- [ ] **Step 2: Implement router**

Routes:

```go
POST /v1/payments
GET /v1/payments/:payment_id
POST /v1/webhooks/robokassa/result
GET /v1/webhooks/robokassa/success
GET /v1/webhooks/robokassa/fail
GET /healthz
GET /readyz
GET /metrics
```

- [ ] **Step 3: Implement create payment handler**

Validate:

- `user_id > 0`;
- `amount_minor > 0`;
- `currency == "RUB"`;
- `description` length is 1..512 runes;
- `idempotency_key` length is 8..128 runes.

- [ ] **Step 4: Implement Robokassa result handler**

Read form values from `application/x-www-form-urlencoded`, build raw payload map, pass to use case, return `text/plain; charset=utf-8`.

- [ ] **Step 5: Verify**

Run:

```powershell
go test ./internal/http -count=1
```

Expected: handler tests pass.

- [ ] **Step 6: Commit**

```powershell
git add internal/http
git commit -m "feat: expose payment http api"
```

---

## Task 11: RabbitMQ Publisher And Outbox Worker

**Files:**
- Create: `internal/adapters/broker/rabbitmq.go`
- Modify: `internal/app/usecase/publish_outbox.go`
- Create: `internal/app/usecase/publish_outbox_test.go`
- Modify: `cmd/outbox-worker/main.go`

- [ ] **Step 1: Write PublishOutbox tests**

Cover:

- pending event publishes and becomes `published`;
- publisher error increments attempts and schedules next retry;
- context cancellation stops loop without marking unpublished events as published;
- event is published to exchange `payments.events` and routing key `payment.succeeded`.

- [ ] **Step 2: Implement RabbitMQ publisher**

Use:

- durable topic exchange `payments.events`;
- persistent messages;
- publisher confirms;
- context-aware publish;
- `Close()` method for channel and connection.

- [ ] **Step 3: Implement use case**

`PublishOutbox` must:

1. Lock batch with `LockPending(ctx, tx, workerID, limit)`.
2. Publish each event.
3. Wait for confirm via broker adapter.
4. Mark published in a transaction after successful confirm.
5. On error, mark failed with `attempts + 1`, `publish_after = now + retryBackoff(attempts)`, and bounded `last_error`.

- [ ] **Step 4: Wire worker command**

`cmd/outbox-worker/main.go` must load config, create logger, DB pool, repositories, RabbitMQ publisher, then run until SIGINT/SIGTERM with graceful shutdown.

- [ ] **Step 5: Verify**

Run:

```powershell
go test ./internal/app/usecase -run TestPublishOutbox -count=1
go test ./internal/adapters/broker -count=1
go test ./cmd/outbox-worker
```

Expected: tests pass.

- [ ] **Step 6: Commit**

```powershell
git add internal/adapters/broker internal/app/usecase/publish_outbox.go internal/app/usecase/publish_outbox_test.go cmd/outbox-worker/main.go
git commit -m "feat: publish outbox events to rabbitmq"
```

---

## Task 12: Reconciliation Worker

**Files:**
- Create: `internal/app/usecase/reconcile_payment.go`
- Create: `internal/app/usecase/reconcile_payment_test.go`
- Create: `cmd/reconciler/main.go`

- [ ] **Step 1: Write tests**

Cover:

- provider says succeeded, use case calls `ConfirmPaymentSucceeded` with reason `reconciliation_success`;
- provider says pending, payment remains `waiting_for_payment`;
- old unpaid payment becomes `expired` with history reason `payment_expired`;
- provider error returns wrapped error and does not update payment.

- [ ] **Step 2: Implement use case**

Use default scan window:

```go
olderThan := now.Add(-5 * time.Minute)
newerThan := now.Add(-72 * time.Hour)
```

For each waiting payment:

- if provider status is `succeeded`, confirm via common use case;
- if `expires_at` is before `now`, mark expired and write status history;
- if provider status is `pending`, leave unchanged.

- [ ] **Step 3: Wire command**

`cmd/reconciler/main.go` runs a ticker, checks `ctx.Done()` between iterations, and exits cleanly on SIGINT/SIGTERM.

- [ ] **Step 4: Verify**

Run:

```powershell
go test ./internal/app/usecase -run TestReconcilePayment -count=1
go test ./cmd/reconciler
```

Expected: tests pass.

- [ ] **Step 5: Commit**

```powershell
git add internal/app/usecase/reconcile_payment.go internal/app/usecase/reconcile_payment_test.go cmd/reconciler/main.go
git commit -m "feat: reconcile pending payments"
```

---

## Task 13: Config, Logging, Metrics, And API Wiring

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/logger/logger.go`
- Create: `internal/observability/metrics.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Implement config**

Read config from environment variables:

```text
HTTP_ADDR
DATABASE_URL
RABBITMQ_URL
ROBOKASSA_MERCHANT_LOGIN
ROBOKASSA_PASSWORD1
ROBOKASSA_PASSWORD2
ROBOKASSA_IS_TEST
ROBOKASSA_BASE_URL
OUTBOX_BATCH_SIZE
OUTBOX_POLL_INTERVAL
RECONCILE_INTERVAL
LOG_LEVEL
```

Validate required values at startup and return one joined error for all missing required fields.

- [ ] **Step 2: Implement slog logger**

Use JSON logs to stdout in production mode, text logs for local mode if `LOG_FORMAT=text`.

- [ ] **Step 3: Implement metrics**

Expose:

- `payments_created_total`;
- `payments_succeeded_total`;
- `robokassa_result_total{signature_valid}`;
- `outbox_events_total{status}`;
- `outbox_publish_duration_seconds`;
- `reconciliation_checked_total`;
- `reconciliation_success_total`;
- HTTP latency histogram by method, route pattern and status.

- [ ] **Step 4: Wire API command**

`cmd/api/main.go` must:

1. Load config.
2. Create logger.
3. Create DB pool.
4. Create repositories.
5. Create Robokassa adapter.
6. Create use cases.
7. Create Gin router.
8. Start `http.Server` with read/write timeouts.
9. Shutdown on SIGINT/SIGTERM with `context.WithTimeout`.

- [ ] **Step 5: Verify**

Run:

```powershell
go test ./internal/config ./internal/logger ./internal/observability ./cmd/api
go test ./...
```

Expected: all unit tests pass.

- [ ] **Step 6: Commit**

```powershell
git add internal/config internal/logger internal/observability cmd/api/main.go
git commit -m "feat: wire api service"
```

---

## Task 14: End-To-End Local Verification

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Document local startup**

Add README commands:

```powershell
docker compose up -d
$env:DATABASE_URL='postgres://payment:payment@localhost:5432/payment?sslmode=disable'
$env:RABBITMQ_URL='amqp://guest:guest@localhost:5672/'
$env:ROBOKASSA_MERCHANT_LOGIN='test'
$env:ROBOKASSA_PASSWORD1='password1'
$env:ROBOKASSA_PASSWORD2='password2'
$env:ROBOKASSA_IS_TEST='true'
make migrate-up
make run-api
make run-outbox
make run-reconciler
```

- [ ] **Step 2: Verify create payment through HTTP**

Run:

```powershell
Invoke-RestMethod -Method Post -Uri http://localhost:8080/v1/payments -ContentType 'application/json' -Body '{
  "user_id": 12345,
  "amount_minor": 29900,
  "currency": "RUB",
  "description": "Покупка 1000 coins",
  "product_code": "coins_1000",
  "idempotency_key": "tg-12345-coins-1000-001"
}'
```

Expected: response contains `status: waiting_for_payment`, `provider: robokassa`, non-empty `payment_url`.

- [ ] **Step 3: Verify duplicate create request**

Run the same command again.

Expected: same `payment_id` and same `provider_invoice_id`.

- [ ] **Step 4: Verify ResultURL callback**

Generate valid Robokassa signature using adapter test helper or a small `go test` fixture, then post form data:

```powershell
Invoke-WebRequest -Method Post -Uri http://localhost:8080/v1/webhooks/robokassa/result -ContentType 'application/x-www-form-urlencoded' -Body 'OutSum=299.00&InvId=10000001&SignatureValue=DE98F9AEE063E4BC6CB7D8E572B764D0'
```

Expected: response body is `OK10000001`; database payment status is `succeeded`; one `PaymentSucceeded` outbox event exists.

- [ ] **Step 5: Verify outbox publishing**

Check RabbitMQ management UI at `http://localhost:15672` or use queue inspection.

Expected: published message body matches `PaymentSucceeded` contract and outbox row status is `published`.

- [ ] **Step 6: Verify failure drills**

Run these drills:

- stop RabbitMQ, confirm payment, start RabbitMQ, verify outbox retry publishes later;
- send same ResultURL twice, verify one outbox event;
- send invalid signature, verify provider event saved with `signature_valid=false` and payment remains `waiting_for_payment`;
- stop API before callback test, run reconciler with provider fake/integration mode, verify successful provider status confirms payment.

- [ ] **Step 7: Run full verification**

Run:

```powershell
make fmt
go test ./...
go test -race ./...
go test -tags=integration ./...
golangci-lint run ./...
```

Expected: all commands pass.

- [ ] **Step 8: Commit**

```powershell
git add README.md
git commit -m "docs: document local payment service verification"
```

---

## Production Readiness Gate

Before production, verify every item:

- [ ] `ResultURL` is the only path that marks payment `succeeded`.
- [ ] `SuccessURL` and `FailURL` only render user-facing result pages.
- [ ] `ResultURL` returns `OK{InvId}` only after PostgreSQL commit.
- [ ] `payments.idempotency_key` unique constraint is present and covered by test.
- [ ] `payment_provider_events` stores raw payload and signature validity.
- [ ] `ConfirmPaymentSucceeded` is shared by callback and reconciler.
- [ ] `outbox_events` unique business event prevents duplicate `PaymentSucceeded`.
- [ ] Outbox worker uses `FOR UPDATE SKIP LOCKED`.
- [ ] RabbitMQ publisher confirms are required before `published`.
- [ ] All DB calls accept and propagate `context.Context`.
- [ ] External Robokassa calls have timeouts.
- [ ] `amount_minor` is `int64`; no money path uses `float64`.
- [ ] Logs do not include Robokassa passwords, database URL password, or raw secrets.
- [ ] Metrics and alerts cover API, outbox and reconciliation failure modes.

## Recommended Execution Order

1. Task 1 through Task 4 create the foundation and should be done sequentially.
2. Task 5 through Task 7 are application-core work and should be reviewed after each task.
3. Task 8 through Task 10 can be parallelized after ports and use cases stabilize.
4. Task 11 and Task 12 can be parallelized after PostgreSQL adapters exist.
5. Task 13 and Task 14 should be last because they wire and verify the full system.

## Self-Review

- Spec coverage: plan maps every architecture section to tasks: domain, migrations, CreatePayment, HandleRobokassaResult, ConfirmPaymentSucceeded, PublishOutbox, ReconcilePayment, HTTP API, RabbitMQ contract, idempotency, metrics, alerts and failure drills.
- Placeholder scan: no task relies on an unspecified future decision; every external boundary has named files, commands and expected observable behavior.
- Type consistency: plan consistently uses `Payment`, `Money`, `StatusWaitingForPayment`, `StatusSucceeded`, `ProviderInvoiceID`, `PaymentSucceeded`, `ports.Tx`, `context.Context`, and repository interfaces under `internal/app/ports/repository`.
