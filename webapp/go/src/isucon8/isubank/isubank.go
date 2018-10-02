// Package isubank is client for ISUBANK API.
package isubank

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
)

var (
	// いすこん銀行にアカウントが存在しない
	ErrNoUser = errors.New("no bank user")

	// 仮決済時または残高チェック時に残高が不足している
	ErrCreditInsufficient = errors.New("credit is insufficient")
)

type isubankResponse interface {
	setStatus(int)
}

type isubankBasicResponse struct {
	status int
	Error  string `json:"error"`
}

type isubankReserveResponse struct {
	isubankBasicResponse
	ReserveID int64 `json:"reserve_id"`
}

func (r *isubankBasicResponse) success() bool {
	return r.status == 200
}

func (r *isubankBasicResponse) setStatus(s int) {
	r.status = s
}

// Isubank はISUBANK APIクライアントです
// NewIsubankによって初期化してください
type Isubank struct {
	endpoint *url.URL
	appID    string
}

// NewIsubank はIsubankを初期化します
//
// endpoint: ISUBANK APIを利用するためのエンドポイントURI
// appID:    ISUBANK APIを利用するためのアプリケーションID
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

// Check は残高確認です
// Reserve による予約済み残高は含まれません
func (b *Isubank) Check(bankID string, price int64) error {
	res := &isubankBasicResponse{}
	v := map[string]interface{}{
		"bank_id": bankID,
		"price":   price,
	}
	if err := b.request("/check", v, res); err != nil {
		return fmt.Errorf("check failed. err: %s", err)
	}
	if res.success() {
		return nil
	}
	if res.Error == "bank_id not found" {
		return ErrNoUser
	}
	if res.Error == "credit is insufficient" {
		return ErrCreditInsufficient
	}
	return fmt.Errorf("check failed. err:%s", res.Error)
}

// Reserve は仮決済(残高の確保)を行います
func (b *Isubank) Reserve(bankID string, price int64) (int64, error) {
	res := &isubankReserveResponse{}
	v := map[string]interface{}{
		"bank_id": bankID,
		"price":   price,
	}
	if err := b.request("/reserve", v, res); err != nil {
		return 0, fmt.Errorf("reserve failed. err: %s", err)
	}
	if !res.success() {
		if res.Error == "credit is insufficient" {
			return 0, ErrCreditInsufficient
		}
		return 0, fmt.Errorf("reserve failed. err:%s", res.Error)
	}
	return res.ReserveID, nil
}

// Commit は決済の確定を行います
// 正常に仮決済処理を行っていればここでエラーになることはありません
func (b *Isubank) Commit(reserveIDs []int64) error {
	res := &isubankBasicResponse{}
	v := map[string]interface{}{
		"reserve_ids": reserveIDs,
	}
	if err := b.request("/commit", v, res); err != nil {
		return fmt.Errorf("commit failed. err: %s", err)
	}
	if !res.success() {
		if res.Error == "credit is insufficient" {
			return ErrCreditInsufficient
		}
		return fmt.Errorf("commit failed. err:%s", res.Error)
	}
	return nil
}

// Cancel は決済の取り消しを行います
func (b *Isubank) Cancel(reserveIDs []int64) error {
	res := &isubankBasicResponse{}
	v := map[string]interface{}{
		"reserve_ids": reserveIDs,
	}
	if err := b.request("/cancel", v, res); err != nil {
		return fmt.Errorf("cancel failed. err: %s", err)
	}
	if !res.success() {
		return fmt.Errorf("cancel failed. err:%s", res.Error)
	}
	return nil
}

func (b *Isubank) request(p string, v interface{}, r isubankResponse) error {
	u := new(url.URL)
	*u = *b.endpoint
	u.Path = path.Join(u.Path, p)

	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(v); err != nil {
		return fmt.Errorf("isubank json encode failed. err: %s", err)
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), body)
	if err != nil {
		return fmt.Errorf("isubank new request failed. err: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.appID)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("isubank request failed. err: %s", err)
	}
	defer res.Body.Close()
	if err = json.NewDecoder(res.Body).Decode(r); err != nil {
		return fmt.Errorf("isubank decode json failed. err: %s", err)
	}
	r.setStatus(res.StatusCode)
	return nil
}
