package http

import "net/http"

type RouterConfig struct {
	PaymentHandler          *PaymentHandler
	RobokassaWebhookHandler *RobokassaWebhookHandler
	HealthHandler           http.Handler
	ReadyHandler            http.Handler
	MetricsHandler          http.Handler
}

func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	if cfg.PaymentHandler != nil {
		mux.HandleFunc("POST /v1/payments", cfg.PaymentHandler.CreatePayment)
		mux.HandleFunc("GET /v1/payments/{payment_id}", cfg.PaymentHandler.GetPayment)
	}

	if cfg.RobokassaWebhookHandler != nil {
		mux.HandleFunc("POST /v1/webhooks/robokassa/result", cfg.RobokassaWebhookHandler.Result)
		mux.HandleFunc("GET /v1/webhooks/robokassa/success", cfg.RobokassaWebhookHandler.Success)
		mux.HandleFunc("GET /v1/webhooks/robokassa/fail", cfg.RobokassaWebhookHandler.Fail)
	}

	mux.Handle("/healthz", handlerOrNotImplemented(cfg.HealthHandler))
	mux.Handle("/readyz", handlerOrNotImplemented(cfg.ReadyHandler))
	mux.Handle("/metrics", handlerOrNotImplemented(cfg.MetricsHandler))

	return mux
}
