package model

import (
	"database/sql"
	"time"

	"github.com/pkg/errors"
)

var (
	ErrBankUserNotFound   = errors.New("bank user not found")
	ErrBankUserConflict   = errors.New("bank user conflict")
	ErrUserNotFound       = errors.New("user not found")
	ErrOrderNotFound      = errors.New("order not found")
	ErrOrderAlreadyClosed = errors.New("order is already closed")
	ErrCreditInsufficient = errors.New("銀行の残高が足りません")
	ErrParameterInvalid   = errors.New("parameter invalid")
	ErrNoOrderForTrade    = errors.New("no order for trade")
)

type RowScanner interface {
	Scan(...interface{}) error
}

type QueryExecuter interface {
	Exec(string, ...interface{}) (sql.Result, error)
	QueryRow(string, ...interface{}) *sql.Row
	Query(string, ...interface{}) (*sql.Rows, error)
}

func InitBenchmark(d QueryExecuter) error {
	var dt time.Time
	if err := d.QueryRow(`select max(created_at) from trade`).Scan(&dt); err != nil {
		return errors.Wrap(err, "get last traded")
	}
	diffmin := int64(time.Now().Sub(dt).Minutes())
	for _, q := range []string{
		"update trade set created_at = (created_at + interval ? minute)",
		"update orders set created_at = (created_at + interval ? minute)",
		"update orders set closed_at = (closed_at + interval ? minute) where closed_at is not null",
	} {
		if _, err := d.Exec("update trade set created_at = (created_at + interval ? minute)", diffmin); err != nil {
			return errors.Wrapf(err, "query exec failed[%d]", q)
		}
	}
	return nil
}
