package http

import (
	"context"
	"net/http"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/usecase"
)

type HandleProviderResultExecutor interface {
	Execute(context.Context, usecase.HandleProviderResultRequest) (usecase.HandleProviderResultResponse, error)
}

type RobokassaWebhookHandler struct {
	handleResult HandleProviderResultExecutor
}

type RobokassaResultRequest struct {
	Values map[string]string
}

type RobokassaResultResponse struct {
	AckBody string
}

func NewRobokassaWebhookHandler(handleResult HandleProviderResultExecutor) *RobokassaWebhookHandler {
	return &RobokassaWebhookHandler{
		handleResult: handleResult,
	}
}

func (h *RobokassaWebhookHandler) Result(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}

func (h *RobokassaWebhookHandler) Success(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}

func (h *RobokassaWebhookHandler) Fail(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}
