package model

import (
	"time"
)

type Trade struct {
	ID        int64     `json:"id"`
	Amount    int64     `json:"amount"`
	Price     int64     `json:"price"`
	CreatedAt time.Time `json:"created_at"`
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

func scanTrade(r RowScanner) (*Trade, error) {
	var v Trade
	if err := r.Scan(&v.ID, &v.Amount, &v.Price, &v.CreatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}
