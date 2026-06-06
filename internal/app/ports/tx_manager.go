package ports

import "context"

type Tx interface{}

type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error
}
