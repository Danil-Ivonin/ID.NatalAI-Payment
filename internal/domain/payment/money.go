package payment

type Money struct {
	AmountMinor int64
	Currency    string
}

func NewMoney(amount int64, currency string) (*Money, error) {
	if amount < 0 {
		return nil, ErrNegativeMoney
	}
	if currency != "RUB" {
		return nil, ErrWrongCurrency
	}
	return &Money{amount, currency}, nil
}
