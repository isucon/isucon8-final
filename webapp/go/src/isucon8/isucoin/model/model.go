package model

import (
	"database/sql"

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
