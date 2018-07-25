package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/pkg/errors"
)

var (
	ErrCreditInsufficient = errors.New("credit is insufficient")
)

type IsubankBasicResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

type IsubankReserveResponse struct {
	IsubankBasicResponse
	ReserveID int64 `json:"reserve_id"`
}

func (r *IsubankBasicResponse) Success() bool {
	return strings.ToLower(r.Status) == "ok"
}

type Isubank struct {
	endpoint *url.URL
	appID    string
}

func NewIsubank(endpoint, appID string) (*Isubank, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	return &Isubank{
		endpoint: u,
		appID:    appID,
	}, nil
}

func (b *Isubank) Check(bankID string, price int64) error {
	res := &IsubankBasicResponse{}
	v := map[string]interface{}{
		"bank_id": bankID,
		"price":   price,
	}
	if err := b.request("/check", v, res); err != nil {
		return errors.Wrap(err, "check failed")
	}
	if !res.Success() {
		if res.Error == "credit is insufficient" {
			return ErrCreditInsufficient
		}
		return errors.Errorf("check failed. err:%s", res.Error)
	}
	return nil
}

func (b *Isubank) Reserve(bankID string, price int64) (int64, error) {
	res := &IsubankReserveResponse{}
	v := map[string]interface{}{
		"bank_id": bankID,
		"price":   price,
	}
	if err := b.request("/reserve", v, res); err != nil {
		return 0, errors.Wrap(err, "reserve failed")
	}
	if !res.Success() {
		if res.Error == "credit is insufficient" {
			return 0, ErrCreditInsufficient
		}
		return 0, errors.Errorf("reserve failed. err:%s", res.Error)
	}
	return res.ReserveID, nil
}

func (b *Isubank) Commit(bankID string, reserveIDs ...int64) error {
	res := &IsubankBasicResponse{}
	v := map[string]interface{}{
		"bank_id":     bankID,
		"reserve_ids": reserveIDs,
	}
	if err := b.request("/commit", v, res); err != nil {
		return errors.Wrap(err, "commit failed")
	}
	if !res.Success() {
		if res.Error == "credit is insufficient" {
			return ErrCreditInsufficient
		}
		return errors.Errorf("commit failed. err:%s", res.Error)
	}
	return nil
}

func (b *Isubank) Cancel(bankID string, reserveIDs ...int64) error {
	res := &IsubankBasicResponse{}
	v := map[string]interface{}{
		"bank_id":     bankID,
		"reserve_ids": reserveIDs,
	}
	if err := b.request("/cancel", v, res); err != nil {
		return errors.Wrap(err, "cancel failed")
	}
	if !res.Success() {
		return errors.Errorf("cancel failed. err:%s", res.Error)
	}
	return nil
}

func (b *Isubank) request(p string, v map[string]interface{}, r interface{}) error {
	u := new(url.URL)
	*u = *b.endpoint
	u.Path = path.Join(u.Path, p)

	v["app_id"] = b.appID
	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(v); err != nil {
		return errors.Wrap(err, "isubank json encode failed")
	}
	res, err := http.Post(u.String(), "application/json", body)
	if err != nil {
		return errors.Wrap(err, "isubank request failed")
	}
	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(r)
}
