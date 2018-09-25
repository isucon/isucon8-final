package isulog

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	TagSignup     = "signup"
	TagSignin     = "signin"
	TagBuyOrder   = "buy.order"
	TagSellOrder  = "sell.order"
	TagBuyError   = "buy.error"
	TagBuyDelete  = "buy.delete"
	TagSellDelete = "sell.delete"
	TagTrade      = "trade"
	TagBuyTrade   = "buy.trade"
	TagSellTrade  = "sell.trade"
)

type Log struct {
	Tag        string          `json:"tag"`
	Time       time.Time       `json:"time"`
	Data       json.RawMessage `json:"data"`
	Signup     *Signup         `json:"-"`
	Signin     *Signin         `json:"-"`
	BuyOrder   *Order          `json:"-"`
	SellOrder  *Order          `json:"-"`
	BuyError   *BuyError       `json:"-"`
	BuyDelete  *OrderDelete    `json:"-"`
	SellDelete *OrderDelete    `json:"-"`
	Trade      *Trade          `json:"-"`
	BuyTrade   *OrderTrade     `json:"-"`
	SellTrade  *OrderTrade     `json:"-"`
}

type Signup struct {
	Name   string `json:"name"`
	BankID string `json:"bank_id"`
	UserID int64  `json:"user_id"`
}

func (d *Signup) Validate() error {
	if d.Name == "" {
		return errors.Errorf("name is empty.")
	}
	if d.BankID == "" {
		return errors.Errorf("bank_id is empty.")
	}
	if d.UserID < 1 {
		return errors.Errorf("user_id is must be upper than 1.")
	}
	return nil
}

type Signin struct {
	UserID int64 `json:"user_id"`
}

func (d *Signin) Validate() error {
	if d.UserID < 1 {
		return errors.Errorf("user_id is must be upper than 1.")
	}
	return nil
}

type Order struct {
	UserID  int64 `json:"user_id"`
	OrderID int64 `json:"order_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

func (d *Order) Validate() error {
	if d.UserID < 1 {
		return errors.Errorf("user_id is must be upper than 1.")
	}
	if d.OrderID < 1 {
		return errors.Errorf("order_id is must be upper than 1.")
	}
	if d.Amount < 1 {
		return errors.Errorf("amount is must be upper than 1.")
	}
	if d.Price < 1 {
		return errors.Errorf("price is must be upper than 1.")
	}
	return nil
}

type BuyError struct {
	UserID int64  `json:"user_id"`
	Amount int64  `json:"amount"`
	Price  int64  `json:"price"`
	Error  string `json:"error"`
}

func (d *BuyError) Validate() error {
	if d.UserID < 1 {
		return errors.Errorf("user_id is must be upper than 1.")
	}
	if d.Amount < 1 {
		return errors.Errorf("amount is must be upper than 1.")
	}
	if d.Price < 1 {
		return errors.Errorf("price is must be upper than 1.")
	}
	if d.Error == "" {
		return errors.Errorf("error is empty.")
	}
	return nil
}

type Trade struct {
	TradeID int64 `json:"trade_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

func (d *Trade) Validate() error {
	if d.TradeID < 1 {
		return errors.Errorf("trade_id is must be upper than 1.")
	}
	if d.Amount < 1 {
		return errors.Errorf("amount is must be upper than 1.")
	}
	if d.Price < 1 {
		return errors.Errorf("price is must be upper than 1.")
	}
	return nil
}

type OrderTrade struct {
	TradeID int64 `json:"trade_id"`
	UserID  int64 `json:"user_id"`
	OrderID int64 `json:"order_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

func (d *OrderTrade) Validate() error {
	if d.TradeID < 1 {
		return errors.Errorf("trade_id is must be upper than 1.")
	}
	if d.UserID < 1 {
		return errors.Errorf("user_id is must be upper than 1.")
	}
	if d.OrderID < 1 {
		return errors.Errorf("order_id is must be upper than 1.")
	}
	if d.Amount < 1 {
		return errors.Errorf("amount is must be upper than 1.")
	}
	if d.Price < 1 {
		return errors.Errorf("price is must be upper than 1.")
	}
	return nil
}

type OrderDelete struct {
	OrderID int64  `json:"order_id"`
	UserID  int64  `json:"user_id"`
	Reason  string `json:"reason"`
}

func (d *OrderDelete) Validate() error {
	if d.UserID < 1 {
		return errors.Errorf("user_id is must be upper than 1.")
	}
	if d.OrderID < 1 {
		return errors.Errorf("order_id is must be upper than 1.")
	}
	if d.Reason != "canceled" && d.Reason != "reserve_failed" {
		return errors.Errorf("reason is must be canceled or reserve_failed.")
	}
	return nil
}

type Isulog struct {
	endpoint *url.URL
	appid    string
}

func NewIsulog(endpoint, appid string) (*Isulog, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	return &Isulog{
		endpoint: u,
		appid:    appid,
	}, nil
}

func (b *Isulog) AppID() string {
	return b.appid
}

func (b *Isulog) Initialize() error {
	u := new(url.URL)
	*u = *b.endpoint
	u.Path = path.Join(u.Path, "/initialize")
	res, err := http.Post(u.String(), "application/json", strings.NewReader("{}"))
	if err != nil {
		return errors.Wrap(err, "isulog /initialize failed")
	}
	defer res.Body.Close()
	if _, err = io.Copy(ioutil.Discard, res.Body); err != nil {
		return errors.Wrap(err, "isulog /initialize failed. body IO")
	}
	if res.StatusCode != 200 {
		return errors.Errorf("isulog /initialize failed. status code [%d]", res.StatusCode)
	}
	return nil

}

func (b *Isulog) GetUserLogs(userID int64) ([]*Log, error) {
	v := url.Values{}
	v.Set("app_id", b.AppID())
	v.Set("user_id", strconv.FormatInt(userID, 10))
	return b.getLogs(v)
}

func (b *Isulog) GetTradeLogs(tradeID int64) ([]*Log, error) {
	v := url.Values{}
	v.Set("app_id", b.AppID())
	v.Set("trade_id", strconv.FormatInt(tradeID, 10))
	return b.getLogs(v)
}

func (b *Isulog) getLogs(v url.Values) ([]*Log, error) {
	u := new(url.URL)
	*u = *b.endpoint
	u.Path = path.Join(u.Path, "/logs")
	u.RawQuery = v.Encode()

	res, err := http.Get(u.String())
	if err != nil {
		return nil, errors.Wrap(err, "isulog GET /logs failed")
	}
	defer res.Body.Close()
	r := []*Log{}
	if err = json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, errors.Wrap(err, "isulog GET /logs decode json failed")
	}
	if err = fetchLogDetails(r); err != nil {
		return nil, err
	}
	return r, nil
}

func fetchLogDetails(logs []*Log) error {
	for _, l := range logs {
		switch l.Tag {
		case TagSignup:
			l.Signup = &Signup{}
			if err := json.Unmarshal(l.Data, l.Signup); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
			if err := l.Signup.Validate(); err != nil {
				return errors.Wrapf(err, "[%s] validation failed.", l.Tag)
			}
		case TagSignin:
			l.Signin = &Signin{}
			if err := json.Unmarshal(l.Data, l.Signin); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
			if err := l.Signin.Validate(); err != nil {
				return errors.Wrapf(err, "[%s] validation failed.", l.Tag)
			}
		case TagBuyOrder:
			l.BuyOrder = &Order{}
			if err := json.Unmarshal(l.Data, l.BuyOrder); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
			if err := l.BuyOrder.Validate(); err != nil {
				return errors.Wrapf(err, "[%s] validation failed.", l.Tag)
			}
		case TagSellOrder:
			l.SellOrder = &Order{}
			if err := json.Unmarshal(l.Data, l.SellOrder); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
			if err := l.SellOrder.Validate(); err != nil {
				return errors.Wrapf(err, "[%s] validation failed.", l.Tag)
			}
		case TagBuyError:
			l.BuyError = &BuyError{}
			if err := json.Unmarshal(l.Data, l.BuyError); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
			if err := l.BuyError.Validate(); err != nil {
				return errors.Wrapf(err, "[%s] validation failed.", l.Tag)
			}
		case TagBuyDelete:
			l.BuyDelete = &OrderDelete{}
			if err := json.Unmarshal(l.Data, l.BuyDelete); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
			if err := l.BuyDelete.Validate(); err != nil {
				return errors.Wrapf(err, "[%s] validation failed.", l.Tag)
			}
		case TagSellDelete:
			l.SellDelete = &OrderDelete{}
			if err := json.Unmarshal(l.Data, l.SellDelete); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
			if err := l.SellDelete.Validate(); err != nil {
				return errors.Wrapf(err, "[%s] validation failed.", l.Tag)
			}
		case TagTrade:
			l.Trade = &Trade{}
			if err := json.Unmarshal(l.Data, l.Trade); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
			if err := l.Trade.Validate(); err != nil {
				return errors.Wrapf(err, "[%s] validation failed.", l.Tag)
			}
		case TagBuyTrade:
			l.BuyTrade = &OrderTrade{}
			if err := json.Unmarshal(l.Data, l.BuyTrade); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
			if err := l.BuyTrade.Validate(); err != nil {
				return errors.Wrapf(err, "[%s] validation failed.", l.Tag)
			}
		case TagSellTrade:
			l.SellTrade = &OrderTrade{}
			if err := json.Unmarshal(l.Data, l.SellTrade); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
			if err := l.SellTrade.Validate(); err != nil {
				return errors.Wrapf(err, "[%s] validation failed.", l.Tag)
			}
		default:
			return errors.Errorf("Unknown tag [%s]", l.Tag)
		}
	}
	return nil
}
