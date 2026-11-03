package postgres

import (
	"context"
	"database/sql"
)

type ctxKey struct{}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// UnitOfWork begins one sql.Tx for aggregate + outbox (instructor-norm).
type UnitOfWork struct {
	DB *sql.DB
}

func NewUnitOfWork(db *sql.DB) *UnitOfWork {
	return &UnitOfWork{DB: db}
}

func (u *UnitOfWork) Within(ctx context.Context, fn func(context.Context) error) error {
	tx, err := u.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	err = fn(context.WithValue(ctx, ctxKey{}, tx))
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func use(ctx context.Context, db *sql.DB) execer {
	if tx, ok := ctx.Value(ctxKey{}).(*sql.Tx); ok && tx != nil {
		return tx
	}
	return db
}
