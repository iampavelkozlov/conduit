package article

import (
	"context"
	"errors"
	"fmt"

	"conduit/internal/gen/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transactionStarter interface {
	Begin(context.Context) (pgx.Tx, error)
}

type PostgresRepository struct {
	postgres.Querier
	pool     transactionStarter
	decorate func(postgres.Querier) postgres.Querier
}

func NewPostgresRepository(
	pool *pgxpool.Pool,
	querier postgres.Querier,
	decorate func(postgres.Querier) postgres.Querier,
) *PostgresRepository {
	return &PostgresRepository{Querier: querier, pool: pool, decorate: decorate}
}

func (r *PostgresRepository) WithinTx(ctx context.Context, operation func(repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	finished := false
	defer func() {
		if !finished {
			_ = tx.Rollback(ctx)
		}
	}()

	txRepository := &PostgresRepository{Querier: r.decorate(postgres.New(tx))}
	if err := operation(txRepository); err != nil {
		finished = true
		return errors.Join(err, tx.Rollback(ctx))
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	finished = true
	return nil
}
