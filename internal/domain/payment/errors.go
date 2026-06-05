package payment

import "errors"

// Money
var ErrNegativeMoney = errors.New("money amount must be greater than zero")
var ErrWrongCurrency = errors.New("wrong currency")

// Order
var ErrPaymentStatus = errors.New("payment status is invalid")
