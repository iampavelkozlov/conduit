package transaction

import (
	"context"
	"errors"
	"fmt"

	"conduit/internal/gen/postgres"

	"github.com/jackc/pgx/v5"
)

type starter interface {
	Begin(context.Context) (pgx.Tx, error)
}

type Transactions struct {
	pool     starter
	decorate func(postgres.Querier) postgres.Querier
}

func New(pool starter, decorate func(postgres.Querier) postgres.Querier) *Transactions {
	return &Transactions{pool: pool, decorate: decorate}
}

func (t *Transactions) WithTx(ctx context.Context, operation func(postgres.Querier) error) error {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	finished := false
	defer func() {
		if !finished {
			_ = tx.Rollback(ctx)
		}
	}()

	repository := t.decorate(postgres.New(tx))
	if err := operation(repository); err != nil {
		finished = true
		return errors.Join(err, tx.Rollback(ctx))
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	finished = true
	return nil
}
