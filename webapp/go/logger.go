package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/pkg/errors"
)

type Log struct {
	Tag  string      `json:"tag"`
	Time int64       `json:"time"`
	Data interface{} `json:"data"`
}

type BulkLog struct {
	AppID string `json:"app_id"`
	Logs  []Log  `json:"logs"`
}

type SoloLog struct {
	Log
	AppID string `json:"app_id"`
}

type LogDataSignup struct {
	Name   string `json:"name"`
	BankID string `json:"bank_id"`
	UserID int64  `json:"user_id"`
}

type LogDataSignin struct {
	UserID int64 `json:"user_id"`
}

type LogDataSellOrder struct {
	UserID int64 `json:"user_id"`
	SellID int64 `json:"sell_id"`
	Amount int64 `json:"amount"`
	Price  int64 `json:"price"`
}

type LogDataBuyOrder struct {
	UserID int64 `json:"user_id"`
	BuyID  int64 `json:"buy_id"`
	Amount int64 `json:"amount"`
	Price  int64 `json:"price"`
}

type LogDataBuyError struct {
	UserID int64  `json:"user_id"`
	Amount int64  `json:"amount"`
	Price  int64  `json:"price"`
	Error  string `json:"error"`
}

type LogDataClose struct {
	TradeID int64 `json:"trade_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

type LogDataSellClose struct {
	TradeID int64 `json:"trade_id"`
	UserID  int64 `json:"user_id"`
	SellID  int64 `json:"sell_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

type LogDataBuyClose struct {
	TradeID int64 `json:"trade_id"`
	UserID  int64 `json:"user_id"`
	BuyID   int64 `json:"buy_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

type Logger struct {
	endpoint *url.URL
	appID    string
}

func NewLogger(endpoint, appID string) (*Logger, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	return &Logger{
		endpoint: u,
		appID:    appID,
	}, nil
}

func (b *Logger) Send(tag string, data interface{}) error {
	v := SoloLog{
		AppID: b.appID,
		Log: Log{
			Tag:  tag,
			Time: time.Now().Unix(),
			Data: data,
		},
	}
	return b.request("/send", v)
}

func (b *Logger) request(p string, v interface{}) error {
	u := new(url.URL)
	*u = *b.endpoint
	u.Path = path.Join(u.Path, p)

	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(v); err != nil {
		return errors.Wrap(err, "logger json encode failed")
	}
	res, err := http.Post(u.String(), "application/json", body)
	if err != nil {
		return errors.Wrap(err, "logger request failed")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return errors.Errorf("logger status is not ok. code: %d", res.StatusCode)
	}
	return nil
}
