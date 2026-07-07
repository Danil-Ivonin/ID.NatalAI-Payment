package grpc

import (
	"context"
	"errors"
	"strings"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/gen/payment"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/usecase"
	domainpayment "github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/domain/payment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreatePaymentExecutor interface {
	Execute(context.Context, usecase.CreatePaymentRequest) (usecase.CreatePaymentResponse, error)
}

type PaymentServer struct {
	payment.UnimplementedPaymentServiceServer
	createPayment CreatePaymentExecutor
}

func NewPaymentServer(createPayment CreatePaymentExecutor) *PaymentServer {
	return &PaymentServer{createPayment: createPayment}
}

func (s *PaymentServer) CreatePayment(
	ctx context.Context,
	req *payment.CreatePaymentRequest,
) (*payment.CreatePaymentResponse, error) {
	if err := validateCreatePaymentRequest(req); err != nil {
		return nil, err
	}
	if s.createPayment == nil {
		return nil, status.Error(codes.Internal, "create payment usecase is not configured")
	}

	return &payment.CreatePaymentResponse{
		PaymentId:         "teest_id",
		Provider:          "denich",
		ProviderInvoiceId: 1234,
		Status:            "ready",
		PaymentUrl:        "PaymentURL",
	}, nil

	resp, err := s.createPayment.Execute(ctx, usecase.CreatePaymentRequest{
		UserID:         req.UserId,
		AmountMinor:    req.AmountMinor,
		Currency:       "RUB",
		Description:    strings.TrimSpace(req.Description),
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	})
	if err != nil {
		return nil, mapCreatePaymentError(err)
	}

	return &payment.CreatePaymentResponse{
		PaymentId:         resp.PaymentID.String(),
		Provider:          resp.Provider,
		ProviderInvoiceId: resp.ProviderInvoiceID,
		Status:            string(resp.Status),
		PaymentUrl:        resp.PaymentURL,
	}, nil
}

func validateCreatePaymentRequest(req *payment.CreatePaymentRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.GetUserId() <= 0 {
		return status.Error(codes.InvalidArgument, "user_id must be greater than zero")
	}
	if req.GetAmountMinor() <= 0 {
		return status.Error(codes.InvalidArgument, "amount_minor must be greater than zero")
	}
	if strings.TrimSpace(req.GetIdempotencyKey()) == "" {
		return status.Error(codes.InvalidArgument, "idempotency_key is required")
	}

	return nil
}

func mapCreatePaymentError(err error) error {
	switch {
	case errors.Is(err, domainpayment.ErrInvalidMoney), errors.Is(err, domainpayment.ErrInvalidCurrency):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domainpayment.ErrPaymentNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domainpayment.ErrAmountMismatch), errors.Is(err, domainpayment.ErrInvalidStatusTransition):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "create payment failed")
	}
}
