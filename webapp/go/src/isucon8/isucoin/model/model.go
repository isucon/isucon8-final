package model

import (
	"database/sql"
	"fmt"
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

type QueryExecutor interface {
	Exec(string, ...interface{}) (sql.Result, error)
	Query(string, ...interface{}) (*sql.Rows, error)
}

func InitBenchmark(d QueryExecutor) error {
	// 前回の10:00:00+0900までのデータを消す
	// 本戦当日は2018-10-20T10:00:00+0900 固定だが、他の時間帯にデータ量を揃える必要がある
	stop := time.Now().Add(-10 * time.Hour)
	stop = time.Date(stop.Year(), stop.Month(), stop.Day(), 10, 0, 0, 0, stop.Location())

	for _, q := range []string{
		fmt.Sprintf("DELETE FROM orders WHERE created_at >= '%s'", stop.Format("2006-01-02 15:00:00")),
		fmt.Sprintf("DELETE FROM trade WHERE created_at >= '%s'", stop.Format("2006-01-02 15:00:00")),
		fmt.Sprintf("DELETE FROM user WHERE created_at >= '%s'", stop.Format("2006-01-02 15:00:00")),
	} {
		if _, err := d.Exec(q); err != nil {
			return errors.Wrapf(err, "query exec failed[%d]", q)
		}
	}
	return nil
}
