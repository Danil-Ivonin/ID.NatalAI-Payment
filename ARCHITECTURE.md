# Архитектура Payment Service

## 1. Назначение сервиса

`payment-service` — микросервис для создания и подтверждения платежей через Robokassa.

Сервис принимает запрос на создание платежа от Telegram-бота, генерирует ссылку на оплату, принимает серверные уведомления от Robokassa, фиксирует успешную оплату в PostgreSQL и публикует событие об успешной оплате через outbox-механизм.

Главная цель сервиса — гарантировать, что пользователь не потеряет деньги даже при сбоях сети, падении приложения, повторных callback-ах или временной недоступности RabbitMQ.

---

## 2. Технологический стек

- Go
- Gin
- PostgreSQL
- pgx
- RabbitMQ
- Robokassa
- golang-migrate
- Hexagonal Architecture

---

## 3. Внешние системы

### Telegram Bot

Telegram-бот обращается в `payment-service`, чтобы создать платёж.

Ответ сервиса содержит ссылку на оплату. Бот отправляет эту ссылку пользователю.

После успешной оплаты бот получает событие `PaymentSucceeded` из RabbitMQ и уведомляет пользователя.

### Robokassa

Robokassa используется как платёжный провайдер.

Сервис взаимодействует с Robokassa в двух направлениях:

1. Генерирует payment URL для оплаты.
2. Принимает `ResultURL` callback после оплаты.

`ResultURL` является серверным подтверждением оплаты. Именно он должен использоваться для изменения статуса платежа на `succeeded`.

`SuccessURL` и `FailURL` можно использовать только для отображения пользователю результата перехода, но не для подтверждения оплаты.

### PostgreSQL

PostgreSQL является source of truth для состояния платежей.

В базе хранятся:

- бизнес-платежи;
- данные платёжного провайдера;
- входящие provider events;
- история изменения статусов;
- outbox events для публикации в RabbitMQ.

### RabbitMQ

RabbitMQ используется для доставки события об успешной оплате в Telegram-бот.

Payment Service публикует событие не напрямую из callback-а, а через transactional outbox.

---

## 4. Общая схема взаимодействия

```text
Telegram Bot
   |
   | POST /v1/payments
   v
Payment Service API
   |
   | INSERT payments
   | INSERT payment_provider_invoices
   v
Telegram Bot получает payment_url
   |
   | отправляет ссылку пользователю
   v
User оплачивает через Robokassa
   |
   v
Robokassa ResultURL
   |
   | POST /v1/webhooks/robokassa/result
   v
Payment Service API
   |
   | verify signature
   | SELECT payment FOR UPDATE
   | UPDATE payment.status = succeeded
   | INSERT payment_provider_events
   | INSERT outbox_events
   | COMMIT
   v
Outbox Worker
   |
   | publish PaymentSucceeded
   v
RabbitMQ
   |
   v
Telegram Bot Consumer
   |
   | idempotency check
   | notify user
   v
User получил подтверждение
```

---

## 5. Hexagonal Architecture

Сервис строится по Hexagonal Architecture.

Основное правило: бизнес-логика не должна зависеть от Gin, pgx, RabbitMQ или Robokassa SDK.

Внутренние слои описывают бизнес-сценарии, а внешние адаптеры реализуют детали инфраструктуры.

---

## 6. Пример структуры проекта

```text
payment-service/
  cmd/
    api/
      main.go
    outbox-worker/
      main.go
    reconciler/
      main.go

  internal/
    domain/
      payment/
        payment.go
        status.go
        money.go
        errors.go
      outbox/
        event.go

    application/
      usecase/
        create_payment.go
        handle_robokassa_result.go
        confirm_payment_succeeded.go
        reconcile_payment.go
        publish_outbox.go
      ports/
        repository/
            payment.go
            provider_invoice.go
            provider_event.go
            status_history.go
            outbox.go
        tx_manager.go
        payment_provider.go
        event_publisher.go

    adapters/
        postgres/
          payment_repository.go
          provider_invoice_repository.go
          provider_event_repository.go
          status_history_repository.go
          outbox_repository.go
          tx_manager.go
    
        robokassa/
          client.go
          signer.go
          url_builder.go
    
        broker/
          rabbit_mq.go
          
    http/
          router.go
          payment_handler.go
          robokassa_webhook_handler.go
          
    config/
      config.go

    logger/
      logger.go

  migrations/
    000001_init.up.sql
    000001_init.down.sql

  go.mod
  go.sum
```

---

## 7. Основные use cases

### 7.1. CreatePayment

Создаёт новый платёж и возвращает ссылку на оплату.

Ответственность:

1. Принять запрос от Telegram-бота.
2. Проверить идемпотентность по `idempotency_key`.
3. Создать запись в `payments`.
4. Создать provider invoice в `payment_provider_invoices`.
5. Сгенерировать Robokassa payment URL.
6. Вернуть payment URL Telegram-боту.

Важно: повторный запрос с тем же `idempotency_key` не должен создавать новый платёж. Он должен вернуть уже существующий платёж.

---

### 7.2. HandleRobokassaResult

Обрабатывает серверный callback от Robokassa.

Ответственность:

1. Принять callback.
2. Сохранить provider event.
3. Проверить подпись Robokassa.
4. Найти платёж по `provider_invoice_id`.
5. Заблокировать платёж через `SELECT ... FOR UPDATE`.
6. Проверить сумму.
7. Если платёж уже `succeeded`, вернуть `OK{InvId}`.
8. Если платёж ещё ожидает оплаты, подтвердить его.
9. Создать outbox event `PaymentSucceeded`.
10. Зафиксировать транзакцию.
11. Вернуть `OK{InvId}`.

Критически важно: ответ `OK{InvId}` должен возвращаться только после успешного commit в PostgreSQL.

---

### 7.3. ConfirmPaymentSucceeded

Внутренний общий use case для подтверждения успешной оплаты.

Его должны использовать:

- `HandleRobokassaResult`;
- `ReconcilePayment`.

Это нужно, чтобы callback и reconciliation не имели разной бизнес-логики.

Ответственность:

1. Заблокировать платёж.
2. Проверить допустимость перехода статуса.
3. Перевести платёж в `succeeded`.
4. Записать историю статуса.
5. Создать outbox event.

---

### 7.4. PublishOutbox

Фоновый worker, который публикует события из `outbox_events` в RabbitMQ.

Ответственность:

1. Найти pending/failed outbox events.
2. Заблокировать пачку событий через `FOR UPDATE SKIP LOCKED`.
3. Опубликовать событие в RabbitMQ.
4. Дождаться publisher confirm.
5. Пометить событие как `published`.
6. В случае ошибки увеличить `attempts` и назначить следующий retry.

---

### 7.5. ReconcilePayment

Фоновая сверка зависших платежей с Robokassa.

Нужна на случай, если пользователь оплатил, но callback от Robokassa не дошёл до сервиса.

Ответственность:

1. Найти платежи в статусе `waiting_for_payment`.
2. Проверить статус платежа у Robokassa.
3. Если Robokassa подтверждает оплату, вызвать `ConfirmPaymentSucceeded`.
4. Если платёж слишком старый и не оплачен, перевести его в `expired`.

---

## 8. Доменная модель

### Payment

`Payment` — основная бизнес-сущность.

Поля по текущей миграции:

```sql
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'RUB',
    description TEXT NOT NULL,
    product_code TEXT,
    status TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    paid_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Назначение полей:

- `id` — внутренний идентификатор платежа.
- `user_id` — id пользователя Telegram или внутренний id пользователя.
- `amount_minor` — сумма в минимальных единицах, например копейках.
- `currency` — валюта платежа.
- `description` — человекочитаемое описание платежа.
- `product_code` — машинный код продукта.
- `status` — текущий статус платежа.
- `idempotency_key` — ключ идемпотентности для защиты от повторного создания платежа.
- `paid_at` — время успешной оплаты.
- `expires_at` — время истечения платежа.

---

## 9. Статусы платежа

```text
created
  |
  v
waiting_for_payment
  |
  | Robokassa ResultURL / Reconciliation success
  v
succeeded
```

Также возможны статусы:

```text
failed
expired
cancelled
refunded
```

Описание:

| Status | Значение |
|---|---|
| `created` | Платёж создан в системе, но ещё не готов к оплате |
| `waiting_for_payment` | Ссылка на оплату создана, ждём оплату |
| `succeeded` | Оплата подтверждена |
| `failed` | Платёж завершился ошибкой |
| `expired` | Истёк срок ожидания оплаты |
| `cancelled` | Платёж отменён |
| `refunded` | Платёж возвращён |

---

## 10. Таблицы базы данных

### 10.1. payments

Главная бизнес-таблица платежей.

Хранит только доменную информацию о платеже.

Не должна превращаться в таблицу Robokassa-specific данных.

---

### 10.2. payment_provider_invoices

Связывает внутренний payment с платёжным провайдером.

```sql
CREATE TABLE payment_provider_invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL REFERENCES payments(id),
    provider TEXT NOT NULL DEFAULT 'robokassa',
    provider_invoice_id BIGINT NOT NULL,
    payment_url TEXT NOT NULL,
    provider_status TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payment_provider_invoice_unique
        UNIQUE (provider, provider_invoice_id),
    CONSTRAINT payment_provider_invoice_payment_unique
        UNIQUE (payment_id, provider)
);
```

Назначение:

- хранит `InvId` Robokassa;
- хранит ссылку на оплату;
- позволяет в будущем добавить других провайдеров без загрязнения таблицы `payments`.

---

### 10.3. payment_provider_events

Хранит все входящие события от платёжного провайдера.

```sql
CREATE TABLE payment_provider_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    provider_invoice_id BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    raw_payload JSONB NOT NULL,
    signature_valid BOOLEAN NOT NULL DEFAULT false,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    CONSTRAINT provider_event_unique UNIQUE (provider, payload_hash)
);
```

Назначение:

- сохранять callback-и Robokassa;
- хранить raw payload для аудита и расследований;
- защищаться от повторной обработки одинакового payload через `payload_hash`;
- видеть, была ли валидной подпись через `signature_valid`.

Важно: `payment_provider_events` не является главным механизмом защиты от двойного начисления. Главная защита — это статус платежа, блокировка строки через `FOR UPDATE`, уникальный outbox event и идемпотентный consumer.

---

### 10.4. payment_status_history

Хранит историю изменения статусов платежа.

```sql
CREATE TABLE payment_status_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL REFERENCES payments(id),
    from_status TEXT,
    to_status TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Назначение:

- аудит изменений статуса;
- расследование спорных платежей;
- понимание, каким путём платёж пришёл в текущее состояние.

Рекомендуется добавить поле:

```sql
reason TEXT NOT NULL
```

Примеры `reason`:

- `payment_created`;
- `payment_url_created`;
- `robokassa_result_url`;
- `reconciliation_success`;
- `payment_expired`;
- `manual_admin_action`;
- `refund_callback`.

---

### 10.5. outbox_events

В текущей миграции таблицы `outbox_events` ещё нет, но для заявленной архитектуры она обязательна.

Рекомендуемая схема:

```sql
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,

    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,

    status TEXT NOT NULL CHECK (
        status IN ('pending', 'processing', 'published', 'failed')
    ) DEFAULT 'pending',

    attempts INT NOT NULL DEFAULT 0,

    publish_after TIMESTAMPTZ NOT NULL DEFAULT now(),

    locked_at TIMESTAMPTZ,
    locked_by TEXT,

    published_at TIMESTAMPTZ,
    last_error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT outbox_unique_business_event UNIQUE (
        aggregate_type,
        aggregate_id,
        event_type
    )
);

CREATE INDEX idx_outbox_pending
    ON outbox_events(status, publish_after, created_at)
    WHERE status IN ('pending', 'failed');
```

Назначение:

- гарантировать, что событие об успешной оплате не потеряется;
- отделить подтверждение платежа от отправки сообщения в RabbitMQ;
- дать возможность retry при недоступности RabbitMQ.

---

## 11. Рекомендуемые дополнительные миграции

### 11.1. Sequence для Robokassa InvId

`provider_invoice_id` для Robokassa удобнее генерировать из sequence.

```sql
CREATE SEQUENCE payment_invoice_id_seq
    START WITH 10000000
    INCREMENT BY 1;
```

При создании платежа:

```sql
SELECT nextval('payment_invoice_id_seq');
```

---

### 11.2. Индексы для payments

В текущей миграции есть индекс по `user_id`, но стоит добавить ещё:

```sql
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_created_at ON payments(created_at);
CREATE INDEX idx_payments_expires_at ON payments(expires_at);
```

Они пригодятся для:

- reconciliation worker;
- поиска зависших платежей;
- истечения старых платежей;
- админки.

---

### 11.3. reason в payment_status_history

```sql
ALTER TABLE payment_status_history
ADD COLUMN reason TEXT NOT NULL DEFAULT 'unknown';
```

Лучше добавить сразу в новой миграции, если проект ещё не в production.

---

## 12. HTTP API

### 12.1. Создание платежа

```http
POST /v1/payments
```

Request:

```json
{
  "user_id": 12345,
  "amount_minor": 29900,
  "currency": "RUB",
  "description": "Покупка 1000 coins",
  "product_code": "coins_1000",
  "idempotency_key": "tg-12345-coins-1000-001"
}
```

Response:

```json
{
  "payment_id": "9b3dbd7e-30f3-49c7-b8b2-492508b7e6fa",
  "provider": "robokassa",
  "provider_invoice_id": 10000001,
  "status": "waiting_for_payment",
  "payment_url": "https://auth.robokassa.ru/Merchant/Index.aspx?..."
}
```

---

### 12.2. Получение платежа

```http
GET /v1/payments/{payment_id}
```

Response:

```json
{
  "payment_id": "9b3dbd7e-30f3-49c7-b8b2-492508b7e6fa",
  "status": "waiting_for_payment",
  "amount_minor": 29900,
  "currency": "RUB",
  "description": "Покупка 1000 coins",
  "product_code": "coins_1000"
}
```

---

### 12.3. Robokassa ResultURL

```http
POST /v1/webhooks/robokassa/result
```

Этот endpoint принимает серверное уведомление от Robokassa.

Обязательная логика:

1. Принять payload.
2. Проверить подпись.
3. Найти платёж.
4. Проверить сумму.
5. Подтвердить платёж.
6. Вернуть `OK{InvId}`.

---

### 12.4. Robokassa SuccessURL

```http
GET /v1/webhooks/robokassa/success
```

Используется только как redirect page для пользователя.

Не должен подтверждать оплату.

---

### 12.5. Robokassa FailURL

```http
GET /v1/webhooks/robokassa/fail
```

Используется только как redirect page для пользователя.

Не должен считаться финальным доказательством неуспешной оплаты, потому что пользователь мог закрыть страницу, вернуться позже или оплата могла находиться в обработке.

---

## 13. Основной flow создания платежа

```text
1. Telegram Bot отправляет POST /v1/payments.
2. Payment Service проверяет idempotency_key.
3. Если такой payment уже существует, сервис возвращает существующий payment_url.
4. Если платежа нет, сервис создаёт новую запись payments.
5. Сервис генерирует provider_invoice_id.
6. Сервис строит Robokassa payment URL.
7. Сервис создаёт payment_provider_invoices.
8. Сервис переводит payment в waiting_for_payment.
9. Telegram Bot получает payment_url.
10. Telegram Bot отправляет ссылку пользователю.
```

---

## 14. Основной flow успешной оплаты

```text
1. Пользователь оплачивает заказ через Robokassa.
2. Robokassa отправляет ResultURL callback.
3. Payment Service проверяет подпись.
4. Payment Service сохраняет payment_provider_event.
5. Payment Service находит payment по provider_invoice_id.
6. Payment Service блокирует payment через SELECT FOR UPDATE.
7. Payment Service проверяет amount_minor и currency.
8. Payment Service переводит payment в succeeded.
9. Payment Service пишет payment_status_history.
10. Payment Service пишет outbox_events.PaymentSucceeded.
11. Payment Service делает COMMIT.
12. Payment Service возвращает OK{InvId}.
13. Outbox Worker публикует событие в RabbitMQ.
14. Telegram Bot Consumer получает событие.
15. Telegram Bot идемпотентно обрабатывает событие.
16. Telegram Bot уведомляет пользователя.
```

---

## 15. Transactional Outbox

Transactional outbox нужен, чтобы не потерять событие об успешной оплате.

Плохой вариант:

```text
1. Обновили payment.status = succeeded.
2. Сервис упал до отправки события в RabbitMQ.
3. Деньги оплачены, но бот ничего не узнал.
```

Правильный вариант:

```text
BEGIN;

UPDATE payments
SET status = 'succeeded'
WHERE id = $payment_id;

INSERT INTO outbox_events (... PaymentSucceeded ...);

COMMIT;
```

Теперь, даже если сервис упадёт после commit, событие останется в PostgreSQL и будет опубликовано позже.

---

## 16. Outbox Worker

Worker должен читать события пачками:

```sql
SELECT *
FROM outbox_events
WHERE status IN ('pending', 'failed')
  AND publish_after <= now()
ORDER BY created_at
LIMIT 100
FOR UPDATE SKIP LOCKED;
```

После публикации в RabbitMQ:

```sql
UPDATE outbox_events
SET status = 'published',
    published_at = now()
WHERE id = $event_id;
```

При ошибке:

```sql
UPDATE outbox_events
SET status = 'failed',
    attempts = attempts + 1,
    publish_after = now() + interval '30 seconds',
    last_error = $error
WHERE id = $event_id;
```

Для RabbitMQ нужно использовать:

- durable exchange;
- durable queue;
- persistent messages;
- publisher confirms;
- manual ack на стороне consumer-а.

---

## 17. RabbitMQ event contract

Exchange:

```text
payments.events
```

Routing key:

```text
payment.succeeded
```

Event:

```json
{
  "event_id": "5f074414-22e8-4c4b-adc0-1d067c7c1c1c",
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

---

## 18. Идемпотентность

### 18.1. Создание платежа

Защита:

```sql
CONSTRAINT payments_idempotency_unique UNIQUE (idempotency_key)
```

Если Telegram-бот повторил запрос из-за timeout, новый платёж не создаётся.

---

### 18.2. Callback от Robokassa

Защита:

- проверка подписи;
- `SELECT ... FOR UPDATE`;
- проверка текущего статуса;
- уникальный outbox event;
- уникальный provider event hash.

---

### 18.3. Consumer Telegram-бота

На стороне Telegram-бота должна быть таблица inbox events:

```sql
CREATE TABLE inbox_events (
    event_id UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Consumer должен сначала записать `event_id`, выполнить бизнес-действие, сделать commit и только потом подтвердить сообщение в RabbitMQ.

---

## 19. Reconciliation

Reconciliation нужен, чтобы защититься от ситуации:

```text
Пользователь оплатил деньги,
но Robokassa callback не дошёл до Payment Service.
```

Reconciler периодически ищет зависшие платежи:

```sql
SELECT p.*, pi.provider_invoice_id
FROM payments p
JOIN payment_provider_invoices pi ON pi.payment_id = p.id
WHERE p.status = 'waiting_for_payment'
  AND p.created_at < now() - interval '5 minutes'
  AND p.created_at > now() - interval '3 days';
```

Для каждого платежа reconciler спрашивает Robokassa о статусе операции.

Если Robokassa подтверждает успешную оплату, reconciler вызывает общий use case:

```text
ConfirmPaymentSucceeded
```

Такой подход не дублирует бизнес-логику callback-а.

---

## 20. Гарантии и сценарии отказа

### Сценарий 1. Telegram Bot повторил запрос создания платежа

Решение:

```text
idempotency_key + unique constraint
```

Результат: пользователь не получит две разные оплаты за один и тот же заказ.

---

### Сценарий 2. Robokassa прислала callback дважды

Решение:

```text
SELECT FOR UPDATE + проверка payment.status
```

Результат: платёж будет подтверждён один раз.

---

### Сценарий 3. Payment Service упал после подтверждения платежа

Решение:

```text
transactional outbox
```

Результат: событие `PaymentSucceeded` останется в PostgreSQL и будет опубликовано после восстановления сервиса.

---

### Сценарий 4. RabbitMQ недоступен

Решение:

```text
outbox retry
```

Результат: платёж не потеряется, событие будет опубликовано позже.

---

### Сценарий 5. Telegram Bot обработал событие, но не отправил ACK

Решение:

```text
inbox_events на стороне Telegram Bot
```

Результат: при повторной доставке событие не будет обработано дважды.

---

### Сценарий 6. Callback от Robokassa не дошёл

Решение:

```text
reconciliation worker
```

Результат: сервис сам найдёт успешную оплату через провайдера и подтвердит её.

---

## 21. Что нельзя делать

Нельзя подтверждать оплату по `SuccessURL`.

```text
SuccessURL != подтверждение оплаты
```

Нельзя сначала отправлять сообщение в RabbitMQ, а потом обновлять статус платежа.

```text
RabbitMQ publish -> DB update
```

Нельзя хранить деньги во `float64`.

```text
Использовать amount_minor BIGINT
```

Нельзя считать, что callback придёт ровно один раз.

```text
Callback может прийти 0, 1 или N раз
```

Нельзя считать публикацию в RabbitMQ финальной, пока не получен publisher confirm.

---

## 22. Минимальный production checklist

Перед запуском в production нужно проверить:

- [ ] Все платежи создаются с `idempotency_key`.
- [ ] `amount_minor` используется вместо float.
- [ ] Robokassa signature проверяется для `ResultURL`.
- [ ] `SuccessURL` не подтверждает оплату.
- [ ] `ResultURL` возвращает `OK{InvId}` только после commit.
- [ ] Callback обработчик использует `SELECT ... FOR UPDATE`.
- [ ] Есть `outbox_events`.
- [ ] Outbox worker использует retry.
- [ ] RabbitMQ publisher confirms включены.
- [ ] Telegram Bot consumer идемпотентен.
- [ ] Есть reconciliation worker.
- [ ] Есть логирование provider events.
- [ ] Есть аудит статусов.
- [ ] Есть алерты по зависшим платежам.

---

## 23. Рекомендуемые метрики

### API

- количество созданных платежей;
- количество успешных платежей;
- количество failed/expired платежей;
- latency создания платежа;
- latency обработки ResultURL;
- количество callback-ов с невалидной подписью.

### Outbox

- количество pending events;
- количество failed events;
- количество retry;
- максимальный возраст pending event;
- latency от `payment.succeeded` до `outbox.published`.

### Reconciliation

- количество проверенных платежей;
- количество найденных успешных платежей;
- количество зависших платежей старше N минут;
- ошибки запросов к Robokassa.

---

## 24. Рекомендуемые алерты

- Есть `waiting_for_payment` старше 30 минут.
- Есть `outbox_events.status = failed` с большим количеством attempts.
- Есть pending outbox events старше 5 минут.
- Резкий рост callback-ов с `signature_valid = false`.
- Robokassa API недоступна для reconciliation.
- RabbitMQ publisher errors.

---

## 25. Ключевое правило архитектуры

Главная гарантия сервиса строится вокруг PostgreSQL transaction:

```text
Robokassa ResultURL
  -> verify signature
  -> BEGIN
       SELECT payment FOR UPDATE
       UPDATE payment.status = succeeded
       INSERT payment_status_history
       INSERT outbox_events.PaymentSucceeded
     COMMIT
  -> return OK{InvId}
```

Если придерживаться этого правила, то даже при падениях сервиса, повторных callback-ах и временной недоступности RabbitMQ деньги пользователя не пропадут.
