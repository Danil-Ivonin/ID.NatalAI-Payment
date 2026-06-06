# Payment Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Go payment-service described in `ARCHITECTURE.md`: Robokassa payment creation, ResultURL confirmation, PostgreSQL source of truth, transactional outbox publishing to RabbitMQ, and reconciliation for missed callbacks.

**Architecture:** Keep the current Hexagonal Architecture. Domain packages stay free of Gin, pgx, RabbitMQ, Robokassa, and environment access. Application use cases depend on ports. Adapters implement ports. `cmd/*/main.go` stays thin and wires dependencies with manual constructor injection.

**Tech Stack:** Go 1.25.5, Gin, pgx/v5, PostgreSQL, RabbitMQ via `amqp091-go`, Robokassa HTTP/signature integration, golang-migrate, `log/slog`, table-driven tests, integration tests behind the `integration` build tag.

---

## Current Repository State

The repository already contains a useful foundation:

- `internal/domain/payment` has `Payment`, `Status`, `Money`, errors, and tests.
- `internal/app/ports` and `internal/app/ports/repository` already define most interfaces and DTOs.
- `migrations/000001_init.up.sql` creates base payment/provider/history tables and `payment_invoice_id_seq`.
- `makefile` already contains `test`, `test-race`, `test-integration`, `lint`, `fmt`, `migrate-up`, `migrate-down`, and run targets.
- `cmd/api/main.go`, `cmd/outbox-worker/main.go`, `internal/app/usecase/*`, `internal/config/config.go`, and `internal/domain/outbox/event.go` are placeholders.
- `go test ./...` currently passes because many packages are empty.

This plan starts from that state. Do not recreate files that already exist. Do not rename `internal/domain/payment/payment.go`; it is already the correct entity file.

## Go Implementation Rules

- Use TDD for domain and usecase behavior: failing test, minimal implementation, passing test.
- Keep package names lowercase and directory-matched.
- Prefer small interfaces at the consumer boundary. Existing ports are the application boundary; adapters must satisfy them.
- Pass `context.Context` to every database, HTTP, RabbitMQ, and worker operation.
- Use pgx with explicit SQL and parameterized placeholders. No ORM.
- Use `SELECT ... FOR UPDATE` when changing payment state after loading a row.
- Use `FOR UPDATE SKIP LOCKED` for outbox batch locking.
- Use `errors.Is`/`errors.As` friendly sentinel errors at domain/application boundaries and wrap technical errors with context.
- Use `amount_minor int64`; no money path may use `float64`.
- External calls must have timeouts.
- Commands in `cmd/*` should parse config, wire dependencies, run, and handle graceful shutdown only.

## File Map

- Modify: `go.mod`, `go.sum` - runtime and test dependencies.
- Modify: `internal/domain/payment/money.go`, `internal/domain/payment/errors.go`, `internal/domain/payment/money_test.go` - align money invariant with DB constraint.
- Modify: `internal/domain/outbox/event.go` - define outbox event entity and statuses.
- Modify: `migrations/000001_init.up.sql`, `migrations/000001_init.down.sql` - add missing outbox table, indexes, and `reason`.
- Modify: `internal/app/ports/*.go`, `internal/app/ports/repository/*.go` - keep existing boundaries, adjust only if usecase tests expose a contract gap.
- Modify: `internal/app/usecase/create_payment.go`, `confirm_payment.go`, `handle_result.go`, `publish_outbox.go`.
- Create: `internal/app/usecase/reconcile_payment.go`.
- Create: `internal/app/usecase/*_test.go`.
- Create: `internal/adapters/postgres/*.go`, `internal/adapters/postgres/*_integration_test.go`.
- Create: `internal/adapters/robokassa/*.go`, `internal/adapters/robokassa/*_test.go`.
- Create: `internal/adapters/broker/rabbitmq.go`, `internal/adapters/broker/rabbitmq_test.go`.
- Create: `internal/http/router.go`, `payment_handler.go`, `robokassa_webhook_handler.go`, `middleware.go`, `*_test.go`.
- Modify: `internal/config/config.go`.
- Create: `internal/logger/logger.go`, `internal/observability/metrics.go`.
- Modify: `cmd/api/main.go`, `cmd/outbox-worker/main.go`.
- Create: `cmd/reconciler/main.go`.
- Modify: `README.md`.

---

## Task 1: Baseline Dependencies And Domain Corrections

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `internal/domain/payment/money.go`
- Modify: `internal/domain/payment/errors.go`
- Modify: `internal/domain/payment/money_test.go`
- Modify: `internal/domain/outbox/event.go`

- [ ] **Step 1: Add dependencies**

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

Expected: `go.mod` has direct requirements for Gin, pgx/v5, amqp091-go, migrate, Prometheus client, and testify if used in tests.

- [ ] **Step 2: Fix money invariant test first**

Change `TestNewMoney` so zero amount is invalid:

```go
{name: "zero amount", amount: 0, currency: "RUB", wantErr: ErrInvalidAmount},
{name: "negative amount", amount: -1, currency: "RUB", wantErr: ErrInvalidAmount},
{name: "empty currency", amount: 100, currency: "", wantErr: ErrInvalidCurrency},
{name: "unsupported currency", amount: 100, currency: "USD", wantErr: ErrInvalidCurrency},
```

Run:

```powershell
go test ./internal/domain/payment -run TestNewMoney -count=1
```

Expected: FAIL until `NewMoney` rejects `amount <= 0` and uses the new error names.

- [ ] **Step 3: Implement money corrections**

Use these sentinel errors in `errors.go`:

```go
var (
	ErrInvalidAmount           = errors.New("invalid amount")
	ErrInvalidCurrency         = errors.New("invalid currency")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrPaymentNotFound         = errors.New("payment not found")
	ErrAmountMismatch          = errors.New("payment amount mismatch")
)
```

Update `NewMoney` to return `Money` by value:

```go
func NewMoney(amountMinor int64, currency string) (Money, error) {
	if amountMinor <= 0 {
		return Money{}, ErrInvalidAmount
	}
	if currency != "RUB" {
		return Money{}, ErrInvalidCurrency
	}
	return Money{AmountMinor: amountMinor, Currency: currency}, nil
}
```

- [ ] **Step 4: Define outbox domain entity**

Replace the placeholder `Event` with:

```go
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
	LastError      *string
	CreatedAt     time.Time
}
```

- [ ] **Step 5: Verify and commit**

Run:

```powershell
gofmt -s -w internal/domain
go test ./internal/domain/... -count=1
go test ./... -count=1
```

Commit:

```powershell
git add go.mod go.sum internal/domain
git commit -m "feat: align domain payment invariants"
```

---

## Task 2: Database Schema Completeness

**Files:**
- Modify: `migrations/000001_init.up.sql`
- Modify: `migrations/000001_init.down.sql`

- [ ] **Step 1: Complete the migration**

Keep existing tables and add:

```sql
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

If production data does not exist, change sequence start to `10000000` to match `ARCHITECTURE.md`; otherwise keep the current start and document the reason in `README.md`.

- [ ] **Step 2: Complete down migration**

Drop in reverse dependency order:

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

Expected: all migration commands complete without SQL errors.

- [ ] **Step 4: Commit**

```powershell
git add migrations/000001_init.up.sql migrations/000001_init.down.sql
git commit -m "feat: complete payment database schema"
```

---

## Task 3: Application Use Cases

**Files:**
- Modify: `internal/app/usecase/create_payment.go`
- Modify: `internal/app/usecase/confirm_payment.go`
- Modify: `internal/app/usecase/handle_result.go`
- Create: `internal/app/usecase/reconcile_payment.go`
- Create: `internal/app/usecase/create_payment_test.go`
- Create: `internal/app/usecase/confirm_payment_test.go`
- Create: `internal/app/usecase/handle_result_test.go`
- Create: `internal/app/usecase/reconcile_payment_test.go`

- [ ] **Step 1: Test `CreatePayment`**

Cover these observable cases with fake repositories and provider:

- new request creates `payments`, gets `NextProviderInvoiceID`, builds Robokassa URL, creates provider invoice, marks status `waiting_for_payment`;
- duplicate `idempotency_key` returns the existing payment and invoice without creating a new payment;
- invalid amount/currency returns a validation/domain error;
- provider URL build failure aborts the transaction.

Run:

```powershell
go test ./internal/app/usecase -run TestCreatePayment -count=1
```

Expected: FAIL until implementation exists.

- [ ] **Step 2: Implement `CreatePayment`**

Use request/response types in `create_payment.go`:

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

Implementation must run inside `TxManager.WithinTx`, use existing repository ports, and return only after the transaction succeeds.

- [ ] **Step 3: Test `ConfirmPaymentSucceeded`**

Cover:

- `waiting_for_payment` becomes `succeeded`;
- status history is written with caller-provided `reason`;
- exactly one `PaymentSucceeded` outbox event is created;
- already `succeeded` is idempotent and does not create a second event;
- amount mismatch returns `payment.ErrAmountMismatch`.

- [ ] **Step 4: Implement `ConfirmPaymentSucceeded`**

The use case must lock the payment through `FindByProviderInvoiceIDForUpdate`, compare amount, update payment, write status history, and insert an outbox event with:

```text
aggregate_type = "payment"
event_type = "PaymentSucceeded"
routing key target = payment.succeeded
```

- [ ] **Step 5: Test and implement `HandleRobokassaResult`**

Tests must cover:

- valid callback persists provider event, verifies signature, confirms payment, marks provider event processed, and returns `OK{InvId}`;
- invalid signature persists provider event with `signature_valid=false` and does not confirm payment;
- duplicate callback for an already succeeded payment returns `OK{InvId}`;
- amount mismatch does not create outbox;
- `OK{InvId}` is produced after `WithinTx` returns nil.

Use SHA-256 over canonical JSON for `payload_hash`. Parse Robokassa `OutSum` decimal into kopecks without `float64`.

- [ ] **Step 6: Test and implement `ReconcilePayment`**

Tests must cover:

- provider says `succeeded`, use case calls shared confirmation with reason `reconciliation_success`;
- provider says `pending`, payment remains unchanged;
- expired unpaid payment becomes `expired` with history reason `payment_expired`;
- provider error is wrapped and does not update payment.

- [ ] **Step 7: Verify and commit**

Run:

```powershell
gofmt -s -w internal/app
go test ./internal/app/... -count=1
go test ./... -count=1
```

Commit:

```powershell
git add internal/app
git commit -m "feat: implement payment use cases"
```

---

## Task 4: PostgreSQL Adapters

**Files:**
- Create: `internal/adapters/postgres/db.go`
- Create: `internal/adapters/postgres/tx_manager.go`
- Create: `internal/adapters/postgres/payment_repository.go`
- Create: `internal/adapters/postgres/provider_invoice_repository.go`
- Create: `internal/adapters/postgres/provider_event_repository.go`
- Create: `internal/adapters/postgres/status_history_repository.go`
- Create: `internal/adapters/postgres/outbox_repository.go`
- Create: `internal/adapters/postgres/*_integration_test.go`

- [ ] **Step 1: Implement pgx pool and transaction manager**

`NewPool(ctx, databaseURL string)` must parse config, set bounded pool values, ping, and return `*pgxpool.Pool`.

`TxManager.WithinTx` must begin, pass concrete `pgx.Tx` as `ports.Tx`, commit on nil, rollback on error, and use `errors.Join` if rollback also fails.

- [ ] **Step 2: Implement repositories with explicit SQL**

Required payment lock query:

```sql
SELECT p.id, p.user_id, p.amount_minor, p.currency, p.description, p.product_code,
       p.status, p.idempotency_key, p.paid_at, p.expires_at, p.created_at, p.updated_at
FROM payments p
JOIN payment_provider_invoices pi ON pi.payment_id = p.id
WHERE pi.provider = $1 AND pi.provider_invoice_id = $2
FOR UPDATE
```

Required outbox lock query:

```sql
SELECT id, aggregate_type, aggregate_id, event_type, payload, status, attempts,
       publish_after, locked_at, locked_by, published_at, last_error, created_at
FROM outbox_events
WHERE status IN ('pending', 'failed')
  AND publish_after <= now()
ORDER BY created_at
LIMIT $1
FOR UPDATE SKIP LOCKED
```

- [ ] **Step 3: Add integration tests**

Use `//go:build integration`. Skip if `TEST_DATABASE_URL` is empty. Cover:

- idempotency unique constraint;
- provider invoice lookup with payment row lock;
- provider event unique `(provider, payload_hash)`;
- outbox unique business event;
- outbox lock batch excludes locked rows.

- [ ] **Step 4: Verify and commit**

Run:

```powershell
go test ./internal/adapters/postgres -count=1
$env:TEST_DATABASE_URL='postgres://payment:payment@localhost:5432/payment?sslmode=disable'
go test -tags=integration ./internal/adapters/postgres -count=1
```

Commit:

```powershell
git add internal/adapters/postgres
git commit -m "feat: add postgres adapters"
```

---

## Task 5: Robokassa Adapter

**Files:**
- Create: `internal/adapters/robokassa/signer.go`
- Create: `internal/adapters/robokassa/url_builder.go`
- Create: `internal/adapters/robokassa/client.go`
- Create: `internal/adapters/robokassa/*_test.go`

- [ ] **Step 1: Test signer and URL builder**

Cover:

- payment URL signature uses password1;
- ResultURL verification uses password2;
- signature comparison is case-insensitive and constant-time after normalization;
- URL uses `net/url.Values` and includes `MerchantLogin`, `OutSum`, `InvId`, `Description`, `SignatureValue`, and `IsTest`.

- [ ] **Step 2: Implement Robokassa provider port**

Implement `ports.PaymentProvider`:

```go
func (c *Client) BuildPaymentURL(req ports.BuildPaymentURLRequest) (string, error)
func (c *Client) VerifyResultSignature(values map[string]string) bool
func (c *Client) CheckPaymentStatus(ctx context.Context, providerInvoiceID int64) (ports.ProviderPaymentStatus, error)
```

HTTP status checks must use `http.NewRequestWithContext` and an `http.Client` with timeout.

- [ ] **Step 3: Verify and commit**

Run:

```powershell
gofmt -s -w internal/adapters/robokassa
go test ./internal/adapters/robokassa -count=1
```

Commit:

```powershell
git add internal/adapters/robokassa
git commit -m "feat: add robokassa adapter"
```

---

## Task 6: HTTP API

**Files:**
- Create: `internal/http/router.go`
- Create: `internal/http/payment_handler.go`
- Create: `internal/http/robokassa_webhook_handler.go`
- Create: `internal/http/middleware.go`
- Create: `internal/http/*_test.go`

- [ ] **Step 1: Test handlers with fakes**

Cover:

- `POST /v1/payments` returns `201` for new payment;
- duplicate create returns `200` with same response;
- validation errors return `400`;
- `POST /v1/webhooks/robokassa/result` returns `text/plain` body `OK10000001`;
- `GET /v1/webhooks/robokassa/success` never confirms payment;
- `GET /v1/webhooks/robokassa/fail` never marks payment failed;
- `/healthz`, `/readyz`, and `/metrics` are registered.

- [ ] **Step 2: Implement routes**

Expose:

```text
POST /v1/payments
GET /v1/payments/:payment_id
POST /v1/webhooks/robokassa/result
GET /v1/webhooks/robokassa/success
GET /v1/webhooks/robokassa/fail
GET /healthz
GET /readyz
GET /metrics
```

Request validation must reject invalid `user_id`, `amount_minor`, `currency`, empty `description`, and short/empty `idempotency_key`.

- [ ] **Step 3: Verify and commit**

Run:

```powershell
gofmt -s -w internal/http
go test ./internal/http -count=1
go test ./... -count=1
```

Commit:

```powershell
git add internal/http
git commit -m "feat: expose payment http api"
```

---

## Task 7: RabbitMQ Publisher And Outbox Worker

**Files:**
- Create: `internal/adapters/broker/rabbitmq.go`
- Modify: `internal/app/usecase/publish_outbox.go`
- Create: `internal/app/usecase/publish_outbox_test.go`
- Modify: `cmd/outbox-worker/main.go`

- [ ] **Step 1: Test `PublishOutbox`**

Cover:

- pending event publishes to exchange `payments.events` with routing key `payment.succeeded`;
- successful publish marks event `published`;
- publisher error marks event `failed`, increments attempts, stores bounded `last_error`, schedules retry;
- context cancellation exits without marking unpublished events as published.

- [ ] **Step 2: Implement RabbitMQ publisher**

Use durable topic exchange, persistent messages, publisher confirms, `PublishWithContext`, and explicit `Close`.

- [ ] **Step 3: Wire outbox worker**

`cmd/outbox-worker/main.go` must load config, create logger, DB pool, repositories, RabbitMQ publisher, run a poll loop, and shut down on SIGINT/SIGTERM.

- [ ] **Step 4: Verify and commit**

Run:

```powershell
gofmt -s -w internal/adapters/broker internal/app/usecase cmd/outbox-worker
go test ./internal/app/usecase -run TestPublishOutbox -count=1
go test ./internal/adapters/broker -count=1
go test ./cmd/outbox-worker -count=1
```

Commit:

```powershell
git add internal/adapters/broker internal/app/usecase/publish_outbox.go internal/app/usecase/publish_outbox_test.go cmd/outbox-worker/main.go
git commit -m "feat: publish payment outbox events"
```

---

## Task 8: Config, Logging, Metrics, API Wiring, Reconciler

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/logger/logger.go`
- Create: `internal/observability/metrics.go`
- Modify: `cmd/api/main.go`
- Create: `cmd/reconciler/main.go`

- [ ] **Step 1: Implement config**

Read and validate:

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
LOG_FORMAT
```

Return one joined validation error for all missing required values.

- [ ] **Step 2: Implement logger and metrics**

Use `log/slog` to stdout. Expose metrics for created/succeeded payments, Robokassa callback result by signature validity, outbox status, publish duration, reconciliation checks, and HTTP latency.

- [ ] **Step 3: Wire API and reconciler commands**

`cmd/api/main.go` wires config, logger, pgx pool, repositories, Robokassa client, use cases, Gin router, `http.Server` timeouts, and graceful shutdown.

`cmd/reconciler/main.go` wires config, logger, pgx pool, repositories, Robokassa client, reconciliation use case, ticker loop, and graceful shutdown.

- [ ] **Step 4: Verify and commit**

Run:

```powershell
gofmt -s -w internal/config internal/logger internal/observability cmd
go test ./internal/config ./internal/logger ./internal/observability ./cmd/... -count=1
go test ./... -count=1
go test -race ./... -count=1
```

Commit:

```powershell
git add internal/config internal/logger internal/observability cmd
git commit -m "feat: wire payment service commands"
```

---

## Task 9: Local End-To-End Verification And Documentation

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Document local startup**

Add commands for Docker, migrations, API, outbox worker, and reconciler:

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

- [ ] **Step 2: Verify HTTP create idempotency**

Run the same `POST /v1/payments` request twice. Expected: the second response returns the same `payment_id` and `provider_invoice_id`.

- [ ] **Step 3: Verify Robokassa ResultURL**

Post a valid form callback. Expected: response body is `OK{InvId}`, payment status is `succeeded`, provider event is processed, and one `PaymentSucceeded` outbox event exists.

- [ ] **Step 4: Verify outbox failure drill**

Stop RabbitMQ, confirm a payment, restart RabbitMQ. Expected: outbox retries and eventually marks the event `published`.

- [ ] **Step 5: Verify duplicate and invalid callback drills**

Send the same valid ResultURL twice. Expected: one successful payment and one outbox event.

Send invalid signature. Expected: provider event saved with `signature_valid=false`, payment remains `waiting_for_payment`, no outbox event is created.

- [ ] **Step 6: Run final verification**

Run:

```powershell
make fmt
go test ./...
go test -race ./...
go test -tags=integration ./...
golangci-lint run ./...
```

Commit:

```powershell
git add README.md
git commit -m "docs: document payment service verification"
```

---

## Production Readiness Gate

Before production, every item must be true:

- [ ] `ResultURL` is the only path that marks payment `succeeded`.
- [ ] `SuccessURL` and `FailURL` only show user-facing result pages.
- [ ] `ResultURL` returns `OK{InvId}` only after PostgreSQL commit.
- [ ] `payments.idempotency_key` has a unique constraint and test coverage.
- [ ] `payment_provider_events` stores raw payload, payload hash, and signature validity.
- [ ] `ConfirmPaymentSucceeded` is shared by callback and reconciler.
- [ ] `outbox_events` has a unique business event constraint.
- [ ] Outbox worker uses `FOR UPDATE SKIP LOCKED`.
- [ ] RabbitMQ publisher confirms are required before marking `published`.
- [ ] All DB calls propagate `context.Context`.
- [ ] External Robokassa calls have timeouts.
- [ ] No money path uses `float64`.
- [ ] Logs never include Robokassa passwords, database password, or raw secrets.
- [ ] Metrics cover API, Robokassa callbacks, outbox, and reconciliation.

## Recommended Execution Order

1. Tasks 1-3 must be sequential: domain, schema, and application contracts set the foundation.
2. Tasks 4-6 can proceed after usecase tests stabilize.
3. Task 7 depends on outbox repository behavior from Task 4.
4. Task 8 should run after adapters and HTTP boundaries compile.
5. Task 9 is the final local verification and documentation pass.

## Self-Review

- Spec coverage: this plan maps the architecture to domain invariants, schema, use cases, PostgreSQL, Robokassa, HTTP, RabbitMQ outbox, reconciliation, config, observability, and local failure drills.
- Placeholder scan: no task contains placeholder markers or unspecified future decisions.
- Type consistency: existing packages and types are preserved where they already exist; new work builds on `payment.Payment`, `payment.Money`, `outbox.Event`, `ports.Tx`, repository ports, and `ports.PaymentProvider`.
