package grpc

import (
	"context"
	"errors"
	"net"
	"testing"

	paymentv1 "github.com/Danil-Ivonin/ID.NatalAI-Payment/gen/payment"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/usecase"
	domainpayment "github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/domain/payment"
	"github.com/google/uuid"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestPaymentServer_CreatePayment(t *testing.T) {
	t.Parallel()

	paymentID := uuid.New()
	tests := []struct {
		name     string
		req      *paymentv1.CreatePaymentRequest
		executor CreatePaymentExecutor
		want     *paymentv1.CreatePaymentResponse
		wantCode codes.Code
	}{
		{
			name: "creates payment",
			req: &paymentv1.CreatePaymentRequest{
				UserId:         42,
				AmountMinor:    29900,
				Description:    "natal report",
				IdempotencyKey: "tg-42-report-1",
			},
			executor: createPaymentExecutorFunc(func(ctx context.Context, req usecase.CreatePaymentRequest) (usecase.CreatePaymentResponse, error) {
				if req.UserID != 42 || req.AmountMinor != 29900 || req.Currency != "RUB" || req.Description != "natal report" {
					t.Fatalf("request = %+v", req)
				}
				return usecase.CreatePaymentResponse{
					PaymentID:         paymentID,
					Provider:          "robokassa",
					ProviderInvoiceID: 7,
					Status:            domainpayment.StatusWaitingForPayment,
					PaymentURL:        "https://pay.example/invoice/7",
				}, nil
			}),
			want: &paymentv1.CreatePaymentResponse{
				PaymentId:         paymentID.String(),
				Provider:          "robokassa",
				ProviderInvoiceId: 7,
				Status:            "waiting_for_payment",
				PaymentUrl:        "https://pay.example/invoice/7",
			},
			wantCode: codes.OK,
		},
		{
			name: "rejects missing user id",
			req: &paymentv1.CreatePaymentRequest{
				AmountMinor:    29900,
				Description:    "natal report",
				IdempotencyKey: "tg-42-report-1",
			},
			executor: createPaymentExecutorFunc(func(ctx context.Context, req usecase.CreatePaymentRequest) (usecase.CreatePaymentResponse, error) {
				t.Fatal("executor must not be called")
				return usecase.CreatePaymentResponse{}, nil
			}),
			wantCode: codes.InvalidArgument,
		},
		{
			name: "maps invalid money",
			req: &paymentv1.CreatePaymentRequest{
				UserId:         42,
				AmountMinor:    29900,
				Description:    "natal report",
				IdempotencyKey: "tg-42-report-1",
			},
			executor: createPaymentExecutorFunc(func(ctx context.Context, req usecase.CreatePaymentRequest) (usecase.CreatePaymentResponse, error) {
				return usecase.CreatePaymentResponse{}, domainpayment.ErrInvalidMoney
			}),
			wantCode: codes.InvalidArgument,
		},
		{
			name: "maps missing payment",
			req: &paymentv1.CreatePaymentRequest{
				UserId:         42,
				AmountMinor:    29900,
				Description:    "natal report",
				IdempotencyKey: "tg-42-report-1",
			},
			executor: createPaymentExecutorFunc(func(ctx context.Context, req usecase.CreatePaymentRequest) (usecase.CreatePaymentResponse, error) {
				return usecase.CreatePaymentResponse{}, domainpayment.ErrPaymentNotFound
			}),
			wantCode: codes.NotFound,
		},
		{
			name: "maps invalid state",
			req: &paymentv1.CreatePaymentRequest{
				UserId:         42,
				AmountMinor:    29900,
				Description:    "natal report",
				IdempotencyKey: "tg-42-report-1",
			},
			executor: createPaymentExecutorFunc(func(ctx context.Context, req usecase.CreatePaymentRequest) (usecase.CreatePaymentResponse, error) {
				return usecase.CreatePaymentResponse{}, domainpayment.ErrInvalidStatusTransition
			}),
			wantCode: codes.FailedPrecondition,
		},
		{
			name: "maps internal error",
			req: &paymentv1.CreatePaymentRequest{
				UserId:         42,
				AmountMinor:    29900,
				Description:    "natal report",
				IdempotencyKey: "tg-42-report-1",
			},
			executor: createPaymentExecutorFunc(func(ctx context.Context, req usecase.CreatePaymentRequest) (usecase.CreatePaymentResponse, error) {
				return usecase.CreatePaymentResponse{}, errors.New("database unavailable")
			}),
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewPaymentServer(tt.executor).CreatePayment(context.Background(), tt.req)
			if status.Code(err) != tt.wantCode {
				t.Fatalf("CreatePayment() code = %s, want %s, err = %v", status.Code(err), tt.wantCode, err)
			}
			if tt.wantCode != codes.OK {
				return
			}
			if got.GetPaymentId() != tt.want.GetPaymentId() ||
				got.GetProvider() != tt.want.GetProvider() ||
				got.GetProviderInvoiceId() != tt.want.GetProviderInvoiceId() ||
				got.GetStatus() != tt.want.GetStatus() ||
				got.GetPaymentUrl() != tt.want.GetPaymentUrl() {
				t.Fatalf("CreatePayment() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestServer_RecoversPanics(t *testing.T) {
	t.Parallel()

	srv := NewServer()
	paymentv1.RegisterPaymentServiceServer(srv, NewPaymentServer(createPaymentExecutorFunc(
		func(ctx context.Context, req usecase.CreatePaymentRequest) (usecase.CreatePaymentResponse, error) {
			panic("boom")
		},
	)))

	lis := bufconn.Listen(1024 * 1024)
	defer lis.Close()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(lis)
	}()
	defer func() {
		GracefulShutdown(srv, 0)
		<-serveErr
	}()

	conn, err := grpcpkg.NewClient(
		"passthrough:///bufnet",
		grpcpkg.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpcpkg.WithInsecure(),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer conn.Close()

	client := paymentv1.NewPaymentServiceClient(conn)
	_, err = client.CreatePayment(context.Background(), &paymentv1.CreatePaymentRequest{
		UserId:         42,
		AmountMinor:    29900,
		Description:    "natal report",
		IdempotencyKey: "tg-42-report-1",
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("CreatePayment() code = %s, want %s, err = %v", status.Code(err), codes.Internal, err)
	}
}

type createPaymentExecutorFunc func(context.Context, usecase.CreatePaymentRequest) (usecase.CreatePaymentResponse, error)

func (f createPaymentExecutorFunc) Execute(ctx context.Context, req usecase.CreatePaymentRequest) (usecase.CreatePaymentResponse, error) {
	return f(ctx, req)
}
