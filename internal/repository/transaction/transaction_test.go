package transaction

import (
	"context"
	"errors"
	"testing"

	"conduit/internal/gen/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestTransactions(t *testing.T) {
	beginErr := errors.New("begin error")
	operationErr := errors.New("operation error")
	rollbackErr := errors.New("rollback error")
	commitErr := errors.New("commit error")
	tests := []struct {
		name       string
		pool       starter
		operation  func(postgres.Querier) error
		wantErr    error
		wantCommit int
		wantRoll   int
	}{
		{name: "begin error", pool: fakeStarter{err: beginErr}, operation: func(postgres.Querier) error { return nil }, wantErr: beginErr},
		{name: "operation error rolls back", pool: &fakeStarter{tx: &fakeTx{rollbackErr: rollbackErr}}, operation: func(postgres.Querier) error { return operationErr }, wantErr: operationErr, wantRoll: 1},
		{name: "commit error rolls back", pool: &fakeStarter{tx: &fakeTx{commitErr: commitErr}}, operation: func(postgres.Querier) error { return nil }, wantErr: commitErr, wantCommit: 1, wantRoll: 1},
		{name: "commits", pool: &fakeStarter{tx: &fakeTx{}}, operation: func(repository postgres.Querier) error { require.NotNil(t, repository); return nil }, wantCommit: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transactions := New(tt.pool, func(querier postgres.Querier) postgres.Querier { return querier })
			err := transactions.WithTx(t.Context(), tt.operation)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			if pool, ok := tt.pool.(*fakeStarter); ok {
				tx, ok := pool.tx.(*fakeTx)
				require.True(t, ok)
				require.Equal(t, tt.wantCommit, tx.commits)
				require.Equal(t, tt.wantRoll, tx.rollbacks)
			}
		})
	}
}

type fakeStarter struct {
	tx  pgx.Tx
	err error
}

func (p fakeStarter) Begin(context.Context) (pgx.Tx, error) {
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
