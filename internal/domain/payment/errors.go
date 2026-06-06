package payment

import "errors"

var (
	ErrNegativeMoney           = errors.New("money amount must be greater than zero")
	ErrWrongCurrency           = errors.New("wrong currency")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrPaymentNotFound         = errors.New("payment not found")
	ErrAmountMismatch          = errors.New("payment amount mismatch")
)
