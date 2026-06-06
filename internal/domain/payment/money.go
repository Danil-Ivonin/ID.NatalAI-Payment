package payment

type Money struct {
	AmountMinor int64
	Currency    string
}

func NewMoney(amount int64, currency string) (*Money, error) {
	if amount <= 0 {
		return nil, ErrInvalidMoney
	}
	if currency != "RUB" {
		return nil, ErrInvalidCurrency
	}
	return &Money{amount, currency}, nil
}
