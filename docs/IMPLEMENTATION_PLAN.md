# Payment Service — Implementation Plan

## Контекст и решения

**gRPC вместо HTTP для клиентов** — Telegram-бот общается с сервисом через gRPC.  
**HTTP остаётся** — только для Robokassa webhook (они шлют HTTP POST, это внешняя система, протокол не выбрать).  
**Один бинарник** — `cmd/api` запускает gRPC-сервер и HTTP-сервер на разных портах.  
**Proto в этом репо** — `proto/payment/v1/payment.proto`, сгенерированный код в `gen/`.

---

## Финальная структура проекта

```
payment-service/
├── proto/
│   └── payment/
│       └── v1/
│           └── payment.proto          # Контракт gRPC API
├── gen/
│   └── payment/
│       └── v1/
│           ├── payment.pb.go          # Сгенерировано (не редактировать)
│           └── payment_grpc.pb.go     # Сгенерировано (не редактировать)
├── cmd/
│   ├── api/
│   │   └── main.go                    # gRPC :50051 + HTTP :8080 (webhooks)
│   ├── outbox-worker/
│   │   └── main.go                    # Фоновый worker outbox
│   └── reconciler/
│       └── main.go                    # Фоновый reconciliation worker
├── internal/
│   ├── domain/
│   │   ├── payment/
│   │   │   ├── payment.go             # Payment entity
│   │   │   ├── status.go              # Status type + переходы
│   │   │   ├── money.go               # Money value object
│   │   │   └── errors.go              # Sentinel errors
│   │   └── outbox/
│   │       └── event.go               # Outbox event entity
│   ├── app/
│   │   ├── ports/
│   │   │   ├── tx_manager.go          # TxManager интерфейс
│   │   │   ├── payment_provider.go    # PaymentProvider интерфейс
│   │   │   ├── event_publisher.go     # EventPublisher интерфейс
│   │   │   └── repository/
│   │   │       ├── payment.go
│   │   │       ├── provider_invoice.go
│   │   │       ├── provider_event.go
│   │   │       ├── status_history.go
│   │   │       └── outbox.go
│   │   └── usecase/
│   │       ├── create_payment.go
│   │       ├── confirm_payment.go
│   │       ├── handle_result.go
│   │       ├── publish_outbox.go
│   │       ├── reconcile_payment.go
│   │       └── errors.go
│   ├── adapters/
│   │   ├── grpc/                      # NEW — gRPC транспорт
│   │   │   ├── server.go              # grpc.Server setup, graceful shutdown
│   │   │   ├── payment_server.go      # PaymentServiceServer impl
│   │   │   └── interceptors.go        # logging, recovery interceptors
│   │   ├── postgres/
│   │   │   ├── db.go                  # pgxpool.Pool factory
│   │   │   ├── tx_manager.go
│   │   │   ├── payment_repository.go
│   │   │   ├── provider_invoice_repository.go
│   │   │   ├── provider_event_repository.go
│   │   │   ├── status_history_repository.go
│   │   │   └── outbox_repository.go
│   │   ├── robokassa/
│   │   │   ├── client.go
│   │   │   ├── signer.go
│   │   │   └── url_builder.go
│   │   └── broker/
│   │       └── rabbitmq.go
│   ├── http/                          # ТОЛЬКО Robokassa + healthz/metrics
│   │   ├── router.go
│   │   ├── robokassa_webhook_handler.go
│   │   └── system_handler.go
│   ├── config/
│   │   └── config.go
│   └── logger/
│       └── logger.go
├── migrations/
│   ├── 000001_init.up.sql
│   └── 000001_init.down.sql
├── proto/                             # см. выше
├── gen/                               # см. выше
├── buf.yaml                           # buf конфигурация
├── buf.gen.yaml                       # генерация кода
├── go.mod
├── go.sum
└── makefile
```

---

## gRPC API Contract

### Методы

| RPC | Запрос | Ответ | Описание |
|-----|--------|-------|----------|
| `CreatePayment` | `CreatePaymentRequest` | `CreatePaymentResponse` | Создать платёж, вернуть URL |

Статус платежа бот узнаёт через RabbitMQ (`PaymentSucceeded`), а не через опрос — `GetPayment` не нужен.

### Proto-схема

```protobuf
// proto/payment/v1/payment.proto
syntax = "proto3";

package payment.v1;

option go_package = "github.com/Danil-Ivonin/ID.NatalAI-Payment/gen/payment/v1;paymentv1";

import "google/protobuf/timestamp.proto";

service PaymentService {
  rpc CreatePayment(CreatePaymentRequest) returns (CreatePaymentResponse);
}

message CreatePaymentRequest {
  int64  user_id         = 1;
  int64  amount_minor    = 2;
  string currency        = 3;
  string description     = 4;
  string product_code    = 5;
  string idempotency_key = 6;
  // expires_at опционально
  google.protobuf.Timestamp expires_at = 7;
}

message CreatePaymentResponse {
  string payment_id          = 1;  // UUID строкой
  string provider            = 2;  // "robokassa"
  int64  provider_invoice_id = 3;
  string status              = 4;
  string payment_url         = 5;
}
```

### gRPC Error Mapping

| Доменная ошибка | gRPC код |
|----------------|----------|
| `ErrInvalidAmount`, `ErrInvalidCurrency` | `InvalidArgument` |
| `ErrPaymentNotFound` | `NotFound` |
| `ErrAmountMismatch` | `FailedPrecondition` |
| `ErrInvalidStatusTransition` | `FailedPrecondition` |
| DB / внутренние | `Internal` |

---

## HTTP API (только для внешних систем)

Robokassa шлёт HTTP — это нельзя изменить. HTTP-сервер остаётся минимальным.

| Метод | Путь | Назначение |
|-------|------|-----------|
| `POST` | `/v1/webhooks/robokassa/result` | Серверный callback Robokassa |
| `GET` | `/v1/webhooks/robokassa/success` | Redirect страница пользователю |
| `GET` | `/v1/webhooks/robokassa/fail` | Redirect страница пользователю |
| `GET` | `/healthz` | Liveness probe |
| `GET` | `/readyz` | Readiness probe |
| `GET` | `/metrics` | Prometheus |

---

## Конфигурация

```
GRPC_ADDR              :50051      # gRPC-сервер
HTTP_ADDR              :8080       # HTTP (webhooks + healthz)
DATABASE_URL           postgres://...
RABBITMQ_URL           amqp://...
ROBOKASSA_MERCHANT_LOGIN
ROBOKASSA_PASSWORD1
ROBOKASSA_PASSWORD2
ROBOKASSA_IS_TEST      true|false
ROBOKASSA_BASE_URL
OUTBOX_BATCH_SIZE      100
OUTBOX_POLL_INTERVAL   5s
RECONCILE_INTERVAL     1m
LOG_LEVEL              info
LOG_FORMAT             json
```

---

## Порядок реализации

### Фаза 1 — Фундамент (последовательно)

#### Задача 1.1: Зависимости и исправления домена

```
go get google.golang.org/grpc@latest
go get google.golang.org/protobuf@latest
go get github.com/gin-gonic/gin@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/rabbitmq/amqp091-go@latest
go get github.com/golang-migrate/migrate/v4@latest
go get github.com/prometheus/client_golang@latest
go get github.com/stretchr/testify@latest
go mod tidy
```

**Изменения в домене:**

- `errors.go` — добавить sentinel errors:  
  `ErrInvalidAmount`, `ErrInvalidCurrency`, `ErrInvalidStatusTransition`,  
  `ErrPaymentNotFound`, `ErrAmountMismatch`
- `money.go` — `NewMoney` возвращает `Money` по значению; отклоняет `amount <= 0`; только `"RUB"`
- `outbox/event.go` — определить `Event` со всеми полями (Status, Attempts, LockedAt, PublishedAt, LastError...)

Верификация: `go test ./internal/domain/... -count=1`

#### Задача 1.2: Схема БД

Добавить в `migrations/000001_init.up.sql`:
- индексы на `payments(status, created_at, expires_at)`
- колонка `reason TEXT NOT NULL DEFAULT 'unknown'` в `payment_status_history`
- таблица `outbox_events` с check-constraint на status и unique constraint на `(aggregate_type, aggregate_id, event_type)`
- индекс `idx_outbox_pending WHERE status IN ('pending','failed')`

Дополнить `migrations/000001_init.down.sql` — drop в обратном порядке.

Верификация: migrate up → down → up без ошибок.

#### Задача 1.3: Use cases

Реализовать последовательно (TDD: тест → реализация):

1. **`CreatePayment`** — idempotency check, `TxManager.WithinTx`, NextProviderInvoiceID, BuildPaymentURL, insert payments + provider_invoice + status_history
2. **`ConfirmPaymentSucceeded`** — `FindByProviderInvoiceIDForUpdate` (SELECT FOR UPDATE), проверка суммы, update status, insert status_history, insert outbox event; идемпотентен если уже `succeeded`
4. **`HandleRobokassaResult`** — save provider event, verify signature, вызвать `ConfirmPaymentSucceeded`, вернуть `OK{InvId}`; `OK{InvId}` только после COMMIT
5. **`PublishOutbox`** — batch lock с SKIP LOCKED, publish, confirm, mark published/failed
6. **`ReconcilePayment`** — ListWaitingForPayment, CheckPaymentStatus, вызвать `ConfirmPaymentSucceeded` или перевести в `expired`

Верификация: `go test ./internal/app/... -count=1`

---

### Фаза 2 — Адаптеры (можно параллельно после Фазы 1)

#### Задача 2.1: PostgreSQL адаптеры

Файлы: `internal/adapters/postgres/`

- `db.go` — `NewPool(ctx, dsn)` с ограниченным пулом, `Ping`
- `tx_manager.go` — `WithinTx`: begin → commit/rollback, `errors.Join` если rollback тоже упал
- `payment_repository.go` — все методы интерфейса, `FOR UPDATE` при lock
- `provider_invoice_repository.go` — `NextProviderInvoiceID()` через sequence
- `provider_event_repository.go`
- `status_history_repository.go`
- `outbox_repository.go` — `LockBatch` с `FOR UPDATE SKIP LOCKED`

Интеграционные тесты (build tag `integration`, skip если `TEST_DATABASE_URL` пуст):
- idempotency unique constraint
- lock batch исключает залоченные строки
- provider event unique по `(provider, payload_hash)`

#### Задача 2.2: Robokassa адаптер

Файлы: `internal/adapters/robokassa/`

- `signer.go` — SHA-256 подпись; password1 для URL, password2 для ResultURL; case-insensitive сравнение constant-time
- `url_builder.go` — `net/url.Values`, без float64 (kopecks как int64)
- `client.go` — реализует `ports.PaymentProvider`; HTTP клиент с таймаутом

#### Задача 2.3: RabbitMQ publisher

Файл: `internal/adapters/broker/rabbitmq.go`

- durable topic exchange `payments.events`
- routing key `payment.succeeded`
- persistent messages
- publisher confirms перед mark published
- `PublishWithContext` для propagation deadline

---

### Фаза 3 — Транспортный слой

#### Задача 3.1: Proto и кодогенерация

```yaml
# buf.yaml
version: v2
deps:
  - buf.build/googleapis/googleapis

# buf.gen.yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: gen
    opt: paths=source_relative
```

```
buf generate
```

Или через `protoc`:
```
protoc --go_out=gen --go-grpc_out=gen \
  --go_opt=paths=source_relative \
  --go-grpc_opt=paths=source_relative \
  proto/payment/v1/payment.proto
```

Добавить в `makefile`:
```makefile
proto-gen:
    buf generate
```

#### Задача 3.2: gRPC сервер

Файлы: `internal/adapters/grpc/`

**`server.go`:**
```go
func NewServer(opts ...grpc.ServerOption) *grpc.Server {
    return grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            recoveryInterceptor,
            loggingInterceptor,
        ),
    )
}

// GracefulShutdown с timeout fallback
func GracefulShutdown(srv *grpc.Server, timeout time.Duration) {
    stopped := make(chan struct{})
    go func() { srv.GracefulStop(); close(stopped) }()
    select {
    case <-stopped:
    case <-time.After(timeout):
        srv.Stop()
    }
}
```

**`payment_server.go`** — реализует `PaymentServiceServer`:
```go
type PaymentServer struct {
    paymentv1.UnimplementedPaymentServiceServer
    createUC *usecase.CreatePaymentUsecase
}

func (s *PaymentServer) CreatePayment(ctx context.Context, req *paymentv1.CreatePaymentRequest) (*paymentv1.CreatePaymentResponse, error) {
    // валидация req полей → codes.InvalidArgument
    // вызов usecase
    // маппинг доменных ошибок → gRPC status codes
}
```

**`interceptors.go`:**
```go
func loggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
    start := time.Now()
    resp, err := handler(ctx, req)
    // slog: method, duration, status code
    return resp, err
}

func recoveryInterceptor(...) { /* panic → codes.Internal */ }
```

Тестирование через `google.golang.org/grpc/test/bufconn` — никакого реального сокета.

#### Задача 3.3: HTTP сервер (только webhooks)

Файлы: `internal/http/`

- `router.go` — регистрирует только Robokassa routes + healthz + metrics
- `robokassa_webhook_handler.go` — `POST /v1/webhooks/robokassa/result`, `GET /v1/webhooks/robokassa/success`, `GET /v1/webhooks/robokassa/fail`
- `system_handler.go` — `/healthz`, `/readyz`, `/metrics`

**Важно:** `POST /v1/webhooks/robokassa/result` возвращает `text/plain` `OK{InvId}` только после COMMIT в Postgres.

---

### Фаза 4 — Сборка и wiring

#### Задача 4.1: Config

```go
type Config struct {
    GRPCAddr    string // GRPC_ADDR, default :50051
    HTTPAddr    string // HTTP_ADDR, default :8080
    DatabaseURL string // required
    RabbitMQURL string // required
    Robokassa   RobokassaConfig
    Outbox      OutboxConfig
    Reconciler  ReconcilerConfig
    Log         LogConfig
}
```

Возвращать один объединённый validation error для всех missing required полей.

#### Задача 4.2: cmd/api/main.go

```
main():
  1. Parse config
  2. Init logger (slog)
  3. Init pgxpool
  4. Init repositories
  5. Init TxManager
  6. Init Robokassa client
  7. Init RabbitMQ publisher (для health check — можно lazy)
  8. Init use cases
  9. Init gRPC server (port :50051)
     - Register PaymentServiceServer
     - Register HealthServer (grpc_health_v1)
  10. Init HTTP server (port :8080)
      - Register Robokassa handlers
      - Register healthz/metrics
  11. Запустить оба сервера в горутинах
  12. Слушать SIGINT/SIGTERM
  13. GracefulShutdown gRPC (15s timeout)
  14. HTTP server.Shutdown(ctx)
  15. Close pgxpool
```

#### Задача 4.3: cmd/outbox-worker/main.go

Poll loop с интервалом `OUTBOX_POLL_INTERVAL`. Graceful shutdown на SIGINT/SIGTERM.

#### Задача 4.4: cmd/reconciler/main.go

Ticker loop с интервалом `RECONCILE_INTERVAL`. Graceful shutdown на SIGINT/SIGTERM.

---

### Фаза 5 — Наблюдаемость и финальная верификация

#### Задача 5.1: Метрики (Prometheus)

| Метрика | Тип | Описание |
|---------|-----|----------|
| `payment_created_total` | Counter | По статусу результата |
| `payment_succeeded_total` | Counter | |
| `robokassa_callback_total` | Counter | По `signature_valid` label |
| `outbox_pending_events` | Gauge | Текущий размер очереди |
| `outbox_publish_duration_seconds` | Histogram | |
| `reconcile_checks_total` | Counter | |
| `grpc_request_duration_seconds` | Histogram | Per method (через interceptor) |

#### Задача 5.2: Финальный прогон

```powershell
go test ./... -count=1
go test -race ./... -count=1
go test -tags=integration ./... -count=1   # требует TEST_DATABASE_URL
golangci-lint run ./...
```

---

## Ключевые инварианты (Production Checklist)

- `OK{InvId}` возвращается только после COMMIT в Postgres
- `SuccessURL` / `FailURL` никогда не меняют статус платежа
- Нет `float64` нигде в денежных путях
- `SELECT ... FOR UPDATE` при обработке callback и reconciliation
- `FOR UPDATE SKIP LOCKED` в outbox worker
- Publisher confirm перед mark `published`
- `ConfirmPaymentSucceeded` — единый shared use case для callback и reconciler
- `outbox_events` имеет unique constraint `(aggregate_type, aggregate_id, event_type)`
- gRPC методы возвращают конкретные `status.Code`, а не raw `error`
- Каждая DB/HTTP/RabbitMQ операция получает `context.Context` с deadline
- gRPC reflection отключён в production

---

## Принятые решения

- **mTLS** — не используется. gRPC работает без TLS (insecure) — при необходимости добавляется позже через `credentials.NewTLS`.
- **gRPC reflection** — отключён.
- **GetPayment** — не нужен. Бот узнаёт об оплате через RabbitMQ событие `PaymentSucceeded`, опрос не требуется.
