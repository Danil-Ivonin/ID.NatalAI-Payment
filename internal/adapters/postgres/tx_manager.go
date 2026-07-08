package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type TxManager struct {
	db txBeginner
}

var _ ports.TxManager = (*TxManager)(nil)

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return newTxManager(pool)
}

func newTxManager(db txBeginner) *TxManager {
	return &TxManager{db: db}
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context, tx ports.Tx) error) error {
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}

		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
