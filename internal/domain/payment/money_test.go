package payment

import (
	"errors"
	"testing"
)

func TestNewMoney(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   int64
		currency string
		wantErr  error
	}{
		{name: "valid rub amount", amount: 29900, currency: "RUB", wantErr: nil},
		{name: "zero amount", amount: 0, currency: "RUB", wantErr: nil},
		{name: "negative amount", amount: -1, currency: "RUB", wantErr: ErrNegativeMoney},
		{name: "empty currency", amount: 100, currency: "", wantErr: ErrWrongCurrency},
		{name: "unsupported currency", amount: 100, currency: "USD", wantErr: ErrWrongCurrency},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewMoney(tt.amount, tt.currency)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewMoney() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("NewMoney() error = %v", err)
				}
				if got.AmountMinor != tt.amount || got.Currency != tt.currency {
					t.Fatalf("NewMoney() = %+v, want amount=%d currency=%s", got, tt.amount, tt.currency)
				}
			}
		})
	}
}
