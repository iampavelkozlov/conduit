package article

import (
	"context"
	"errors"
	"testing"

	"conduit/internal/gen/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresRepository(t *testing.T) {
	repository := NewPostgresRepository(nil, nil, func(querier postgres.Querier) postgres.Querier { return querier })
	require.NotNil(t, repository)
}

func TestWithinTx(t *testing.T) {
	beginErr := errors.New("begin error")
	operationErr := errors.New("operation error")
	rollbackErr := errors.New("rollback error")
	commitErr := errors.New("commit error")
	tests := []struct {
		name       string
		pool       transactionStarter
		operation  func(repository) error
		wantErr    error
		wantCommit int
		wantRoll   int
	}{
		{name: "begin error", pool: fakeTransactionStarter{err: beginErr}, operation: func(repository) error { return nil }, wantErr: beginErr},
		{name: "operation error rolls back", pool: &fakeTransactionStarter{tx: &fakeTx{rollbackErr: rollbackErr}}, operation: func(repository) error { return operationErr }, wantErr: operationErr, wantRoll: 1},
		{name: "commit error rolls back", pool: &fakeTransactionStarter{tx: &fakeTx{commitErr: commitErr}}, operation: func(repository) error { return nil }, wantErr: commitErr, wantCommit: 1, wantRoll: 1},
		{name: "commits", pool: &fakeTransactionStarter{tx: &fakeTx{}}, operation: func(repository) error { return nil }, wantCommit: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &PostgresRepository{
				pool: tt.pool,
				decorate: func(querier postgres.Querier) postgres.Querier {
					return querier
				},
			}
			err := repository.WithinTx(t.Context(), tt.operation)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			if pool, ok := tt.pool.(*fakeTransactionStarter); ok {
				tx, ok := pool.tx.(*fakeTx)
				require.True(t, ok)
				require.Equal(t, tt.wantCommit, tx.commits)
				require.Equal(t, tt.wantRoll, tx.rollbacks)
			}
		})
	}
}

type fakeTransactionStarter struct {
	tx  pgx.Tx
	err error
}

func (p fakeTransactionStarter) Begin(context.Context) (pgx.Tx, error) {
	return p.tx, p.err
}

type fakeTx struct {
	pgx.Tx
	commitErr   error
	rollbackErr error
	commits     int
	rollbacks   int
}

func (tx *fakeTx) Commit(context.Context) error {
	tx.commits++
	return tx.commitErr
}

func (tx *fakeTx) Rollback(context.Context) error {
	tx.rollbacks++
	return tx.rollbackErr
}
