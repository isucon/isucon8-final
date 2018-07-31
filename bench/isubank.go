package bench

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/pkg/errors"
)

type IsubankBasicResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func (r *IsubankBasicResponse) Success() bool {
	return strings.ToLower(r.Status) == "ok"
}

type Isubank struct {
	endpoint *url.URL
}

func NewIsubank(endpoint string) (*Isubank, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	return &Isubank{
		endpoint: u,
	}, nil
}

func (b *Isubank) NewBankID(bankid string) (string, error) {
	var res IsubankBasicResponse
	if err := b.request("/register", map[string]interface{}{"bank_id": bankid}, &res); err != nil {
		return err
	}
	if res.Success() {
		return nil
	}
	return errors.Errorf("/register failed. %s", res.Error)
}

func (b *Isubank) AddCredit(bankid string, price int64) error {
	var res IsubankBasicResponse
	if err := b.request("/add_credit", map[string]interface{}{"bank_id": bankid, "price": price}, &res); err != nil {
		return err
	}
	if res.Success() {
		return nil
	}
	return errors.Error("failed add credit. bankid:%s, price:%d, err:%s", bankid, price, res.Error)
}

func (b *Isubank) request(p string, v map[string]interface{}, r interface{}) error {
	u := new(url.URL)
	*u = *b.endpoint
	u.Path = path.Join(u.Path, p)

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
