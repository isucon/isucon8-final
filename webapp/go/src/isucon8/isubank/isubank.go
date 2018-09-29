package isubank

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"path"

	"github.com/pkg/errors"
)

var (
	ErrNoUser             = errors.New("no bank user")
	ErrCreditInsufficient = errors.New("credit is insufficient")
)

type isubankResponse interface {
	SetStatus(int)
}

type isubankBasicResponse struct {
	status int
	Error  string `json:"error"`
}

type isubankReserveResponse struct {
	isubankBasicResponse
	ReserveID int64 `json:"reserve_id"`
}

func (r *isubankBasicResponse) Success() bool {
	return r.status == 200
}

func (r *isubankBasicResponse) SetStatus(s int) {
	r.status = s
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
	res := &isubankBasicResponse{}
	v := map[string]interface{}{
		"bank_id": bankID,
		"price":   price,
	}
	if err := b.request("/check", v, res); err != nil {
		return errors.Wrap(err, "check failed")
	}
	if res.Success() {
		return nil
	}
	if res.Error == "bank_id not found" {
		return ErrNoUser
	}
	if res.Error == "credit is insufficient" {
		return ErrCreditInsufficient
	}
	return errors.Errorf("check failed. err:%s", res.Error)
}

func (b *Isubank) Reserve(bankID string, price int64) (int64, error) {
	res := &isubankReserveResponse{}
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

func (b *Isubank) Commit(reserveIDs []int64) error {
	res := &isubankBasicResponse{}
	v := map[string]interface{}{
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

func (b *Isubank) Cancel(reserveIDs []int64) error {
	res := &isubankBasicResponse{}
	v := map[string]interface{}{
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

func (b *Isubank) request(p string, v interface{}, r isubankResponse) error {
	u := new(url.URL)
	*u = *b.endpoint
	u.Path = path.Join(u.Path, p)

	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(v); err != nil {
		return errors.Wrap(err, "isubank json encode failed")
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), body)
	if err != nil {
		return errors.Wrap(err, "isubank new request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.appID)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "isubank request failed")
	}
	defer res.Body.Close()
	if err = json.NewDecoder(res.Body).Decode(r); err != nil {
		return errors.Wrap(err, "isubank decode json failed")
	}
	r.SetStatus(res.StatusCode)
	return nil
}
