package model

import (
	"database/sql"

	"github.com/go-sql-driver/mysql"
)

func scanCandlestickDatas(rows *sql.Rows, e error) (candlestickDatas []*CandlestickData, err error) {
	if e != nil {
		return nil, e
	}
	defer func() {
		err = rows.Close()
	}()
	candlestickDatas = []*CandlestickData{}
	for rows.Next() {
		var v CandlestickData
		if err = rows.Scan(&v.Time, &v.Open, &v.Close, &v.High, &v.Low); err != nil {
			return
		}
		candlestickDatas = append(candlestickDatas, &v)
	}
	err = rows.Err()
	return
}

func scanCandlestickData(rows *sql.Rows, err error) (*CandlestickData, error) {
	v, err := scanCandlestickDatas(rows, err)
	if err != nil {
		return nil, err
	}
	if len(v) > 0 {
		return v[0], nil
	}
	return nil, sql.ErrNoRows
}

func scanOrders(rows *sql.Rows, e error) (orders []*Order, err error) {
	if e != nil {
		return nil, e
	}
	defer func() {
		err = rows.Close()
	}()
	orders = []*Order{}
	for rows.Next() {
		var v Order
		var closedAt mysql.NullTime
		var tradeID sql.NullInt64
		if err = rows.Scan(&v.ID, &v.Type, &v.UserID, &v.Amount, &v.Price, &closedAt, &tradeID, &v.CreatedAt); err != nil {
			return nil, err
		}
		if closedAt.Valid {
			v.ClosedAt = &closedAt.Time
		}
		if tradeID.Valid {
			v.TradeID = tradeID.Int64
		}
		orders = append(orders, &v)
	}
	err = rows.Err()
	return
}

func scanOrder(rows *sql.Rows, err error) (*Order, error) {
	v, err := scanOrders(rows, err)
	if err != nil {
		return nil, err
	}
	if len(v) > 0 {
		return v[0], nil
	}
	return nil, sql.ErrNoRows
}

func scanSettings(rows *sql.Rows, e error) (settings []*Setting, err error) {
	if e != nil {
		return nil, e
	}
	defer func() {
		err = rows.Close()
	}()
	settings = []*Setting{}
	for rows.Next() {
		var v Setting
		if err = rows.Scan(&v.Name, &v.Val); err != nil {
			return
		}
		settings = append(settings, &v)
	}
	err = rows.Err()
	return
}

func scanSetting(rows *sql.Rows, err error) (*Setting, error) {
	v, err := scanSettings(rows, err)
	if err != nil {
		return nil, err
	}
	if len(v) > 0 {
		return v[0], nil
	}
	return nil, sql.ErrNoRows
}

func scanTrades(rows *sql.Rows, e error) (trades []*Trade, err error) {
	if e != nil {
		return nil, e
	}
	defer func() {
		err = rows.Close()
	}()
	trades = []*Trade{}
	for rows.Next() {
		var v Trade
		if err = rows.Scan(&v.ID, &v.Amount, &v.Price, &v.CreatedAt); err != nil {
			return
		}
		trades = append(trades, &v)
	}
	err = rows.Err()
	return
}

func scanTrade(rows *sql.Rows, err error) (*Trade, error) {
	v, err := scanTrades(rows, err)
	if err != nil {
		return nil, err
	}
	if len(v) > 0 {
		return v[0], nil
	}
	return nil, sql.ErrNoRows
}

func scanUsers(rows *sql.Rows, e error) (users []*User, err error) {
	if e != nil {
		return nil, e
	}
	defer func() {
		err = rows.Close()
	}()
	users = []*User{}
	for rows.Next() {
		var v User
		if err = rows.Scan(&v.ID, &v.BankID, &v.Name, &v.Password, &v.CreatedAt); err != nil {
			return
		}
		users = append(users, &v)
	}
	err = rows.Err()
	return
}

func scanUser(rows *sql.Rows, err error) (*User, error) {
	v, err := scanUsers(rows, err)
	if err != nil {
		return nil, err
	}
	if len(v) > 0 {
		return v[0], nil
	}
	return nil, sql.ErrNoRows
}
