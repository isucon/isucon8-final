package isubank

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"github.com/pkg/errors"
)

type isubankResponse interface {
	SetStatus(int)
}

type isubankBasicResponse struct {
	status int
	Error  string `json:"error"`
}

func (r *isubankBasicResponse) Success() bool {
	return r.status == 200
}

func (r *isubankBasicResponse) SetStatus(s int) {
	r.status = s
}

type Isubank struct {
	endpoint *url.URL
	appid    string
}

func NewIsubank(endpoint, appid string) (*Isubank, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	return &Isubank{
		endpoint: u,
		appid:    appid,
	}, nil
}

func (b *Isubank) AppID() string {
	return b.appid
}

func (b *Isubank) NewBankID(bankid string) error {
	var res isubankBasicResponse
	if err := b.request("/register", map[string]interface{}{"bank_id": bankid}, &res); err != nil {
		return err
	}
	if res.Success() {
		return nil
	}
	return errors.Errorf("/register failed. %s", res.Error)
}

func (b *Isubank) AddCredit(bankid string, price int64) error {
	var res isubankBasicResponse
	if err := b.request("/add_credit", map[string]interface{}{"bank_id": bankid, "price": price}, &res); err != nil {
		return err
	}
	if res.Success() {
		return nil
	}
	return errors.Errorf("failed add credit. bankid:%s, price:%d, err:%s", bankid, price, res.Error)
}

func (b *Isubank) GetCredit(bankid string) (int64, error) {
	u := new(url.URL)
	*u = *b.endpoint
	u.Path = path.Join(u.Path, "/credit")
	u.RawQuery = url.Values{"bank_id": []string{bankid}}.Encode()
	res, err := http.Get(u.String())
	if err != nil {
		return 0, errors.Wrap(err, "isubank get_credit failed")
	}
	defer res.Body.Close()
	if res.StatusCode == 200 {
		type Res struct {
			Credit int64 `json:"credit"`
		}
		var r Res
		if err = json.NewDecoder(res.Body).Decode(&r); err != nil {
			return 0, errors.Wrap(err, "isubank get_credit decode failed")
		}
		return r.Credit, nil
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return 0, errors.Wrap(err, "isubank read body failed")
	}
	return 0, errors.Errorf("isubank getCredit failed. [status:%d, body:%s]", res.StatusCode, string(body))
}

func (b *Isubank) request(p string, v map[string]interface{}, r isubankResponse) error {
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
	if err = json.NewDecoder(res.Body).Decode(r); err != nil {
		return errors.Wrap(err, "isubank decode json failed")
	}
	r.SetStatus(res.StatusCode)
	return nil
}
