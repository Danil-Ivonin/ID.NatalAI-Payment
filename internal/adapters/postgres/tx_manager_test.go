package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestTxManagerWithinTxCommitsWhenCallbackSucceeds(t *testing.T) {
	ctx := context.Background()
	tx := &fakePGXTx{}
	manager := newTxManager(fakeTxBeginner{tx: tx})

	var gotTx ports.Tx
	err := manager.WithinTx(ctx, func(ctx context.Context, tx ports.Tx) error {
		gotTx = tx
		return nil
	})

	if err != nil {
		t.Fatalf("WithinTx returned error: %v", err)
	}
	if gotTx != tx {
		t.Fatalf("callback got tx %T, want fake tx", gotTx)
	}
	if tx.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", tx.commitCalls)
	}
	if tx.rollbackCalls != 0 {
		t.Fatalf("rollback calls = %d, want 0", tx.rollbackCalls)
	}
}

func TestTxManagerWithinTxRollsBackWhenCallbackFails(t *testing.T) {
	ctx := context.Background()
	callbackErr := errors.New("callback failed")
	tx := &fakePGXTx{}
	manager := newTxManager(fakeTxBeginner{tx: tx})

	err := manager.WithinTx(ctx, func(ctx context.Context, tx ports.Tx) error {
		return callbackErr
	})

	if !errors.Is(err, callbackErr) {
		t.Fatalf("WithinTx error = %v, want callback error", err)
	}
	if tx.commitCalls != 0 {
		t.Fatalf("commit calls = %d, want 0", tx.commitCalls)
	}
	if tx.rollbackCalls != 1 {
		t.Fatalf("rollback calls = %d, want 1", tx.rollbackCalls)
	}
}

func TestTxManagerWithinTxJoinsCallbackAndRollbackErrors(t *testing.T) {
	ctx := context.Background()
	callbackErr := errors.New("callback failed")
	rollbackErr := errors.New("rollback failed")
	tx := &fakePGXTx{rollbackErr: rollbackErr}
	manager := newTxManager(fakeTxBeginner{tx: tx})

	err := manager.WithinTx(ctx, func(ctx context.Context, tx ports.Tx) error {
		return callbackErr
	})

	if !errors.Is(err, callbackErr) {
		t.Fatalf("WithinTx error = %v, want callback error", err)
	}
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("WithinTx error = %v, want rollback error", err)
	}
}

func TestTxManagerWithinTxReturnsBeginError(t *testing.T) {
	ctx := context.Background()
	beginErr := errors.New("begin failed")
	manager := newTxManager(fakeTxBeginner{err: beginErr})

	err := manager.WithinTx(ctx, func(ctx context.Context, tx ports.Tx) error {
		t.Fatal("callback must not be called when begin fails")
		return nil
	})

	if !errors.Is(err, beginErr) {
		t.Fatalf("WithinTx error = %v, want begin error", err)
	}
}

func TestTxManagerWithinTxReturnsCommitError(t *testing.T) {
	ctx := context.Background()
	commitErr := errors.New("commit failed")
	tx := &fakePGXTx{commitErr: commitErr}
	manager := newTxManager(fakeTxBeginner{tx: tx})

	err := manager.WithinTx(ctx, func(ctx context.Context, tx ports.Tx) error {
		return nil
	})

	if !errors.Is(err, commitErr) {
		t.Fatalf("WithinTx error = %v, want commit error", err)
	}
	if tx.rollbackCalls != 0 {
		t.Fatalf("rollback calls = %d, want 0", tx.rollbackCalls)
	}
}

type fakeTxBeginner struct {
	tx  pgx.Tx
	err error
}

func (b fakeTxBeginner) Begin(context.Context) (pgx.Tx, error) {
	return b.tx, b.err
}

type fakePGXTx struct {
	commitCalls   int
	rollbackCalls int
	commitErr     error
	rollbackErr   error
}

func (tx *fakePGXTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("nested transactions are not supported by fakePGXTx")
}

func (tx *fakePGXTx) Commit(context.Context) error {
	tx.commitCalls++
	return tx.commitErr
}

func (tx *fakePGXTx) Rollback(context.Context) error {
	tx.rollbackCalls++
	return tx.rollbackErr
}

func (*fakePGXTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("not implemented")
}

func (*fakePGXTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	panic("not implemented")
}

func (*fakePGXTx) LargeObjects() pgx.LargeObjects {
	panic("not implemented")
}

func (*fakePGXTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("not implemented")
}

func (*fakePGXTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	panic("not implemented")
}

func (*fakePGXTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("not implemented")
}

func (*fakePGXTx) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("not implemented")
}

func (*fakePGXTx) Conn() *pgx.Conn {
	panic("not implemented")
}
