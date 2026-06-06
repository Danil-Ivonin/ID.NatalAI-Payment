package payment

type Status string

const (
	StatusCreated           Status = "created"
	StatusWaitingForPayment Status = "waiting_for_payment"
	StatusSucceeded         Status = "succeeded"
	StatusFailed            Status = "failed"
	StatusExpired           Status = "expired"
	StatusCancelled         Status = "cancelled"
	StatusRefunded          Status = "refunded"
)
