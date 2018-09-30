package model

import (
	"database/sql"
	"time"

	"github.com/go-sql-driver/mysql"
)

type Trade struct {
	ID        int64     `json:"id"`
	Amount    int64     `json:"amount"`
	Price     int64     `json:"price"`
	CreatedAt time.Time `json:"created_at"`
}

type Order struct {
	ID        int64      `json:"id"`
	Type      string     `json:"type"`
	UserID    int64      `json:"user_id"`
	Amount    int64      `json:"amount"`
	Price     int64      `json:"price"`
	ClosedAt  *time.Time `json:"closed_at"`
	TradeID   int64      `json:"trade_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	User      *User      `json:"user,omitempty"`
	Trade     *Trade     `json:"trade,omitempty"`
}

type CandlestickData struct {
	Time  time.Time `json:"time"`
	Open  int64     `json:"open"`
	Close int64     `json:"close"`
	High  int64     `json:"high"`
	Low   int64     `json:"low"`
}

type RowScanner interface {
	Scan(...interface{}) error
}

func scanUser(r RowScanner) (*User, error) {
	var v User
	if err := r.Scan(&v.ID, &v.BankID, &v.Name, &v.Password, &v.CreatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}

func scanTrade(r RowScanner) (*Trade, error) {
	var v Trade
	if err := r.Scan(&v.ID, &v.Amount, &v.Price, &v.CreatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}

func scanOrder(r RowScanner) (*Order, error) {
	var v Order
	var closedAt mysql.NullTime
	var tradeID sql.NullInt64
	if err := r.Scan(&v.ID, &v.Type, &v.UserID, &v.Amount, &v.Price, &closedAt, &tradeID, &v.CreatedAt); err != nil {
		return nil, err
	}
	if closedAt.Valid {
		v.ClosedAt = &closedAt.Time
	}
	if tradeID.Valid {
		v.TradeID = tradeID.Int64
	}
	return &v, nil
}
