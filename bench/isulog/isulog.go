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

type Signin struct {
	UserID int64 `json:"user_id"`
}

type Order struct {
	UserID  int64 `json:"user_id"`
	OrderID int64 `json:"order_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

type BuyError struct {
	UserID int64  `json:"user_id"`
	Amount int64  `json:"amount"`
	Price  int64  `json:"price"`
	Error  string `json:"error"`
}

type Trade struct {
	TradeID int64 `json:"trade_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

type OrderTrade struct {
	TradeID int64 `json:"trade_id"`
	UserID  int64 `json:"user_id"`
	OrderID int64 `json:"order_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

type OrderDelete struct {
	OrderID int64  `json:"order_id"`
	UserID  int64  `json:"user_id"`
	Reason  string `json:"reason"`
}

type Isulog struct {
	endpoint *url.URL
}

func NewIsulog(endpoint string) (*Isulog, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	return &Isulog{
		endpoint: u,
	}, nil
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
	if err = io.Copy(res.Body, ioutil.Discard); err != nil {
		return errors.Wrap(err, "isulog /initialize failed. body IO")
	}
	if res.StatusCode != 200 {
		return errors.Errorf("isulog /initialize failed. status code [%d]", res.StatusCode)
	}
	return nil

}

func (b *Isulog) GetUserLogs(appID string, userID int64) ([]*Log, error) {
	v := url.Values{}
	v.Set("app_id", appID)
	v.Set("user_id", strconv.FormatInt(userID, 10))
	return b.getLogs(v)
}

func (b *Isulog) GetTradeLogs(appID string, tradeID int64) ([]*Log, error) {
	v := url.Values{}
	v.Set("app_id", appID)
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
			if l.Signup.Name == "" {
				return errors.Wrapf(err, "[%s] name is empty.", l.Tag)
			}
			if l.Signup.BankID == "" {
				return errors.Wrapf(err, "[%s] bank_id is empty.", l.Tag)
			}
			if l.Signup.UserID < 1 {
				return errors.Wrapf(err, "[%s] user_id is must be upper than 1.", l.Tag)
			}
		case TagSignin:
			l.Signin = &Signin{}
			if err := json.Unmarshal(l.Data, l.Signin); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
		case TagBuyOrder:
			l.BuyOrder = &Order{}
			if err := json.Unmarshal(l.Data, l.BuyOrder); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
		case TagSellOrder:
			l.SellOrder = &Order{}
			if err := json.Unmarshal(l.Data, l.SellOrder); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
		case TagBuyError:
			l.BuyError = &BuyError{}
			if err := json.Unmarshal(l.Data, l.BuyError); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
		case TagBuyDelete:
			l.BuyDelete = &OrderDelete{}
			if err := json.Unmarshal(l.Data, l.BuyDelete); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
		case TagSellDelete:
			l.SellDelete = &OrderDelete{}
			if err := json.Unmarshal(l.Data, l.SellDelete); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
		case TagTrade:
			l.Trade = &Trade{}
			if err := json.Unmarshal(l.Data, l.Trade); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
		case TagBuyTrade:
			l.BuyTrade = &OrderTrade{}
			if err := json.Unmarshal(l.Data, l.BuyTrade); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
		case TagSellTrade:
			l.SellTrade = &OrderTrade{}
			if err := json.Unmarshal(l.Data, l.SellTrade); err != nil {
				return errors.Wrapf(err, "[%s] parse failed.", l.Tag)
			}
		default:
			return errors.Errorf("Unknown tag [%s]", l.Tag)
		}
	}
	return nil
}
