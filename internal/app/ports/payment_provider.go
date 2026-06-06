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
