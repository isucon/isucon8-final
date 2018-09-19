package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/pkg/errors"
)

type Log struct {
	Tag  string      `json:"tag"`
	Time time.Time   `json:"time"`
	Data interface{} `json:"data"`
}

type LogDataSignup struct {
	Name   string `json:"name"`
	BankID string `json:"bank_id"`
	UserID int64  `json:"user_id"`
}

type LogDataSignin struct {
	UserID int64 `json:"user_id"`
}

type LogDataOrder struct {
	UserID  int64 `json:"user_id"`
	OrderID int64 `json:"order_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

type LogDataBuyError struct {
	UserID int64  `json:"user_id"`
	Amount int64  `json:"amount"`
	Price  int64  `json:"price"`
	Error  string `json:"error"`
}

type LogDataTrade struct {
	TradeID int64 `json:"trade_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

type LogDataOrderTrade struct {
	TradeID int64 `json:"trade_id"`
	UserID  int64 `json:"user_id"`
	OrderID int64 `json:"order_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

type LogDataOrderDelete struct {
	OrderID int64  `json:"order_id"`
	Reason  string `json:"reason"`
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
	return b.request("/send", Log{
		Tag:  tag,
		Time: time.Now(),
		Data: data,
	})
}

func (b *Logger) request(p string, v interface{}) error {
	u := new(url.URL)
	*u = *b.endpoint
	u.Path = path.Join(u.Path, p)

	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(v); err != nil {
		return errors.Wrap(err, "logger json encode failed")
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), body)
	if err != nil {
		return errors.Wrap(err, "logger new request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "app_id "+b.appID)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "logger request failed")
	}
	defer res.Body.Close()
	bo, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "logger body read failed")
	}
	if res.StatusCode == http.StatusOK {
		return nil
	}
	return errors.Errorf("logger status is not ok. code: %d, body: %s", res.StatusCode, string(bo))
}
