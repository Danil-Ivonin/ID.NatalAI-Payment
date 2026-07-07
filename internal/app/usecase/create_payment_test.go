package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports/repository"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/domain/payment"
	"github.com/google/uuid"
)

func TestCreatePaymentUsecase_ExecuteCreatesPayment(t *testing.T) {
	t.Parallel()

	paymentRepo := &fakePaymentRepo{}
	providerInvoiceRepo := &fakeProviderInvoiceRepo{nextProviderInvoiceID: 1001}
	statusHistoryRepo := &fakeStatusHistoryRepo{}
	provider := &fakePaymentProvider{paymentURL: "https://auth.robokassa.ru/Merchant/Index.aspx?InvId=1001"}
	uc := NewCreatePaymentUsecase(
		fakeTxManager{},
		provider,
		paymentRepo,
		providerInvoiceRepo,
		statusHistoryRepo,
	)

	got, err := uc.Execute(context.Background(), CreatePaymentRequest{
		UserID:         42,
		AmountMinor:    29900,
		Currency:       "RUB",
		Description:    "natal report",
		IdempotencyKey: "tg-42-report-1",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got.PaymentID == uuid.Nil ||
		got.Provider != "robokassa" ||
		got.ProviderInvoiceID != 1001 ||
		got.Status != payment.StatusWaitingForPayment ||
		got.PaymentURL != provider.paymentURL {
		t.Fatalf("Execute() = %+v", got)
	}
	if len(paymentRepo.created) != 1 {
		t.Fatalf("created payments = %d, want 1", len(paymentRepo.created))
	}
	if len(providerInvoiceRepo.created) != 1 {
		t.Fatalf("created invoices = %d, want 1", len(providerInvoiceRepo.created))
	}
	if len(statusHistoryRepo.created) != 1 {
		t.Fatalf("created status history = %d, want 1", len(statusHistoryRepo.created))
	}
	history := statusHistoryRepo.created[0]
	if history.FromStatus == nil || *history.FromStatus != payment.StatusCreated || history.ToStatus != payment.StatusWaitingForPayment {
		t.Fatalf("status history = %+v", history)
	}
	if provider.requests != 1 {
		t.Fatalf("provider requests = %d, want 1", provider.requests)
	}
}

func TestCreatePaymentUsecase_ExecuteReturnsExistingPaymentForIdempotencyKey(t *testing.T) {
	t.Parallel()

	paymentID := uuid.New()
	paymentRepo := &fakePaymentRepo{
		existingByIdempotencyKey: map[string]payment.Payment{
			"tg-42-report-1": {
				ID:             paymentID,
				UserID:         42,
				Amount:         payment.Money{AmountMinor: 29900, Currency: "RUB"},
				Description:    "natal report",
				Status:         payment.StatusWaitingForPayment,
				IdempotencyKey: "tg-42-report-1",
			},
		},
	}
	providerInvoiceRepo := &fakeProviderInvoiceRepo{
		existingByPaymentID: map[uuid.UUID]repository.ProviderInvoice{
			paymentID: {
				PaymentID:         paymentID,
				Provider:          "robokassa",
				ProviderInvoiceID: 1001,
				PaymentURL:        "https://pay.example/1001",
			},
		},
	}
	statusHistoryRepo := &fakeStatusHistoryRepo{}
	provider := &fakePaymentProvider{}
	uc := NewCreatePaymentUsecase(fakeTxManager{}, provider, paymentRepo, providerInvoiceRepo, statusHistoryRepo)

	got, err := uc.Execute(context.Background(), CreatePaymentRequest{
		UserID:         42,
		AmountMinor:    29900,
		Currency:       "RUB",
		Description:    "natal report",
		IdempotencyKey: "tg-42-report-1",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got.PaymentID != paymentID || got.ProviderInvoiceID != 1001 || got.PaymentURL != "https://pay.example/1001" {
		t.Fatalf("Execute() = %+v", got)
	}
	if len(paymentRepo.created) != 0 {
		t.Fatalf("created payments = %d, want 0", len(paymentRepo.created))
	}
	if provider.requests != 0 {
		t.Fatalf("provider requests = %d, want 0", provider.requests)
	}
	if len(statusHistoryRepo.created) != 0 {
		t.Fatalf("created status history = %d, want 0", len(statusHistoryRepo.created))
	}
}

func TestCreatePaymentUsecase_ExecuteAbortsWhenProviderURLBuildFails(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("robokassa unavailable")
	paymentRepo := &fakePaymentRepo{}
	providerInvoiceRepo := &fakeProviderInvoiceRepo{nextProviderInvoiceID: 1001}
	statusHistoryRepo := &fakeStatusHistoryRepo{}
	provider := &fakePaymentProvider{err: wantErr}
	uc := NewCreatePaymentUsecase(fakeTxManager{}, provider, paymentRepo, providerInvoiceRepo, statusHistoryRepo)

	_, err := uc.Execute(context.Background(), CreatePaymentRequest{
		UserID:         42,
		AmountMinor:    29900,
		Currency:       "RUB",
		Description:    "natal report",
		IdempotencyKey: "tg-42-report-1",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
	if len(providerInvoiceRepo.created) != 0 {
		t.Fatalf("created invoices = %d, want 0", len(providerInvoiceRepo.created))
	}
	if len(statusHistoryRepo.created) != 0 {
		t.Fatalf("created status history = %d, want 0", len(statusHistoryRepo.created))
	}
}

type fakeTxManager struct{}

func (fakeTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context, tx ports.Tx) error) error {
	return fn(ctx, struct{}{})
}

type fakePaymentProvider struct {
	paymentURL string
	err        error
	requests   int
}

func (p *fakePaymentProvider) BuildPaymentURL(req ports.BuildPaymentURLRequest) (string, error) {
	p.requests++
	if p.err != nil {
		return "", p.err
	}
	return p.paymentURL, nil
}

func (p *fakePaymentProvider) VerifyResultSignature(map[string]string) bool {
	return false
}

func (p *fakePaymentProvider) CheckPaymentStatus(context.Context, int64) (ports.ProviderPaymentStatus, error) {
	return ports.ProviderPaymentStatusPending, nil
}

type fakePaymentRepo struct {
	existingByIdempotencyKey map[string]payment.Payment
	created                  []payment.Payment
}

func (r *fakePaymentRepo) Create(ctx context.Context, tx ports.Tx, p payment.Payment) (payment.Payment, error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	r.created = append(r.created, p)
	return p, nil
}

func (r *fakePaymentRepo) FindByID(context.Context, ports.Tx, uuid.UUID) (payment.Payment, error) {
	return payment.Payment{}, payment.ErrPaymentNotFound
}

func (r *fakePaymentRepo) FindByIdempotencyKey(ctx context.Context, tx ports.Tx, key string) (payment.Payment, bool, error) {
	p, ok := r.existingByIdempotencyKey[key]
	return p, ok, nil
}

func (r *fakePaymentRepo) FindByProviderInvoiceIDForUpdate(context.Context, ports.Tx, string, int64) (payment.Payment, error) {
	return payment.Payment{}, payment.ErrPaymentNotFound
}

func (r *fakePaymentRepo) Update(context.Context, ports.Tx, payment.Payment) error {
	return nil
}

func (r *fakePaymentRepo) ListWaitingForPayment(context.Context, ports.Tx, time.Time, time.Time, int) ([]payment.Payment, error) {
	return nil, nil
}

type fakeProviderInvoiceRepo struct {
	nextProviderInvoiceID int64
	existingByPaymentID   map[uuid.UUID]repository.ProviderInvoice
	created               []repository.ProviderInvoice
}

func (r *fakeProviderInvoiceRepo) NextProviderInvoiceID(context.Context, ports.Tx) (int64, error) {
	return r.nextProviderInvoiceID, nil
}

func (r *fakeProviderInvoiceRepo) Create(ctx context.Context, tx ports.Tx, invoice repository.ProviderInvoice) error {
	r.created = append(r.created, invoice)
	return nil
}

func (r *fakeProviderInvoiceRepo) FindByPaymentID(ctx context.Context, tx ports.Tx, paymentID uuid.UUID, provider string) (repository.ProviderInvoice, error) {
	invoice, ok := r.existingByPaymentID[paymentID]
	if !ok {
		return repository.ProviderInvoice{}, payment.ErrPaymentNotFound
	}
	return invoice, nil
}

type fakeStatusHistoryRepo struct {
	created []repository.StatusHistory
}

func (r *fakeStatusHistoryRepo) Create(ctx context.Context, tx ports.Tx, history repository.StatusHistory) error {
	r.created = append(r.created, history)
	return nil
}
