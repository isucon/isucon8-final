package model

import (
	"database/sql"
	"fmt"
	"strings"
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

type QueryExecutor interface {
	Exec(string, ...interface{}) (sql.Result, error)
	QueryRow(string, ...interface{}) *sql.Row
	Query(string, ...interface{}) (*sql.Rows, error)
}

func InitBenchmark(d QueryExecutor) error {
	var dt time.Time
	if err := d.QueryRow(`select max(created_at) from orders`).Scan(&dt); err != nil {
		return errors.Wrap(err, "get last traded")
	}
	// 前回の10:00:00+0900までのデータを消す
	stop := time.Now()
	if stop.Hour() >= 10 {
		stop = time.Date(stop.Year(), stop.Month(), stop.Day(), 10, 0, 0, 0, stop.Location())
	} else {
		stop = time.Date(stop.Year(), stop.Month(), stop.Day()-1, 10, 0, 0, 0, stop.Location())
	}
	p := []string{}
	for dt.After(stop) {
		p = append(p, dt.Format("p2006010215"))
	}

	diffmin := int64(time.Now().Sub(dt).Minutes())
	for _, q := range []string{
		fmt.Sprintf("ALTER TABLE orders TRUNCATE PARTITION %s", strings.Join(p, ",")),
		fmt.Sprintf("ALTER TABLE trade TRUNCATE PARTITION %s", strings.Join(p, ",")),
		fmt.Sprintf("DELETE FROM user WHERE created_at >= '%s'", stop.Format("2006-01-02 15:00:00")),
	} {
		if _, err := d.Exec(q, diffmin); err != nil {
			return errors.Wrapf(err, "query exec failed[%d]", q)
		}
	}
	return nil
}
