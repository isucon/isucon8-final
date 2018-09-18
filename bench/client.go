package bench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/publicsuffix"
)

var (
	UserAgent      = "Isutrader/0.0.1"
	createdAtUpper = time.Now().Add(24 * time.Hour).Unix()
)

const (
	TradeTypeSell = "sell"
	TradeTypeBuy  = "buy"
)

func init() {
	var err error
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Panicln(err)
	}
	time.Local = loc
}

type ResponseWithElapsedTime struct {
	*http.Response
	ElapsedTime time.Duration
}

type ErrorWithStatus struct {
	StatusCode int
	Body       string
	err        error
}

func errorWithStatus(err error, code int, body string) *ErrorWithStatus {
	return &ErrorWithStatus{
		StatusCode: code,
		Body:       body,
		err:        err,
	}
}

func (e *ErrorWithStatus) Error() string {
	return fmt.Sprintf("%s [status:%d, body:%s]", e.err.Error(), e.StatusCode, e.Body)
}

type StatusRes struct {
	OK    bool   `jon:"ok"`
	Error string `jon:"error,omitempty"`
}

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	BankID    string    `json:"-"`
	CreatedAt time.Time `json:"-"`
}

type Trade struct {
	ID        int64     `json:"id"`
	Amount    int64     `json:"amount"`
	Price     int64     `json:"price"`
	CreatedAt time.Time `json:"created_at"`
}

type Order struct {
	ID        int64      `json:"id"`
	Type      string     `json:"type"`
	UserID    int64      `json:"user_id"`
	Amount    int64      `json:"amount"`
	Price     int64      `json:"price"`
	ClosedAt  *time.Time `json:"closed_at"`
	TradeID   int64      `json:"trade_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	User      *User      `json:"user,omitempty"`
	Trade     *Trade     `json:"trade,omitempty"`
}

type InfoResponse struct {
	TradedOrders    []Order `json:"traded_orders"`
	Trades          []Trade `json:"trades"`
	LowestSellPrice int64   `json:"lowest_sell_price"`
	HighestBuyPrice int64   `json:"highest_buy_price"`
}

type OrderActionResponse struct {
	ID int64 `json:"id"`
}

type Client struct {
	base   *url.URL
	hc     *http.Client
	bankid string
	pass   string
	name   string
	cache  *CacheStore
}

func NewClient(base, bankid, name, password string, timout time.Duration) (*Client, error) {
	b, err := url.Parse(base)
	if err != nil {
		return nil, errors.Wrapf(err, "base url parse Failed.")
	}
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, errors.Wrapf(err, "cookiejar.New Failed.")
	}
	hc := &http.Client{
		Jar: jar,
		// Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: timout,
	}
	return &Client{
		base:   b,
		hc:     hc,
		bankid: bankid,
		name:   name,
		pass:   password,
		cache:  NewCacheStore(),
	}, nil
}

func (c *Client) doRequest(req *http.Request) (*ResponseWithElapsedTime, error) {
	req.Header.Set("User-Agent", UserAgent)
	start := time.Now()
	res, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	elapsedTime := time.Now().Sub(start)
	return &ResponseWithElapsedTime{res, elapsedTime}, nil
}

func (c *Client) get(path string, val url.Values) (*ResponseWithElapsedTime, error) {
	u, err := c.base.Parse(path)
	if err != nil {
		return nil, errors.Wrap(err, "url parse failed")
	}
	for k, v := range u.Query() {
		val[k] = v
	}
	u.RawQuery = val.Encode()
	us := u.String()
	req, err := http.NewRequest(http.MethodGet, us, nil)
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	if cache, found := c.cache.Get(us); found {
		// no-storeを外しかつcache-controlをつければOK
		if cache.CanUseCache() {
			return &ResponseWithElapsedTime{
				Response: &http.Response{
					StatusCode: http.StatusNotModified,
					Body:       ioutil.NopCloser(&bytes.Buffer{}),
				},
				ElapsedTime: 0,
			}, nil
		}
		cache.ApplyRequest(req)
	}
	res, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	if cache, ok := NewURLCache(res.Response); ok {
		c.cache.Set(us, cache)
	}
	return res, nil
}

func (c *Client) post(path string, val url.Values) (*ResponseWithElapsedTime, error) {
	u, err := c.base.Parse(path)
	if err != nil {
		return nil, errors.Wrap(err, "url parse failed")
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(val.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.doRequest(req)
}

func (c *Client) del(path string, val url.Values) (*ResponseWithElapsedTime, error) {
	u, err := c.base.Parse(path)
	if err != nil {
		return nil, errors.Wrap(err, "url parse failed")
	}
	for k, v := range u.Query() {
		val[k] = v
	}
	u.RawQuery = val.Encode()
	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	return c.doRequest(req)
}

func (c *Client) Initialize(bankep, bankid, logep, logid string) error {
	v := url.Values{}
	v.Set("bank_endpoint", bankep)
	v.Set("bank_appid", bankid)
	v.Set("log_endpoint", logep)
	v.Set("log_appid", logid)
	res, err := c.post("/initialize", v)
	if err != nil {
		return errors.Wrap(err, "POST /initialize request failed")
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "POST /initialize body read failed")
	}
	if res.StatusCode == http.StatusOK {
		return nil
	}
	return errorWithStatus(errors.Errorf("POST /initialize failed."), res.StatusCode, string(b))
}

func (c *Client) Signup() error {
	v := url.Values{}
	v.Set("name", c.name)
	v.Set("bank_id", c.bankid)
	v.Set("password", c.pass)
	res, err := c.post("/signup", v)
	if err != nil {
		return errors.Wrap(err, "POST /signup request failed")
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "POST /signup body read failed")
	}
	if res.StatusCode == http.StatusOK {
		return nil
	}
	return errorWithStatus(errors.Errorf("POST /signup failed."), res.StatusCode, string(b))
}

func (c *Client) Signin() error {
	v := url.Values{}
	v.Set("bank_id", c.bankid)
	v.Set("password", c.pass)
	res, err := c.post("/signin", v)
	if err != nil {
		return errors.Wrap(err, "POST /signin request failed")
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "POST /signin body read failed")
	}
	if res.StatusCode == http.StatusOK {
		return nil
	}
	return errorWithStatus(errors.Errorf("POST /signin failed."), res.StatusCode, string(b))
}

func (c *Client) Top() error {
	res, err := c.get("/", url.Values{})
	if err != nil {
		return errors.Wrap(err, "GET / request failed")
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "GET / body read failed")
	}
	if res.StatusCode == http.StatusOK {
		return nil
	}
	// TODO top HTML の不正チェック
	return errorWithStatus(errors.Errorf("GET / failed."), res.StatusCode, string(b))
}

func (c *Client) Info(lastID int64) (*InfoResponse, error) {
	path := "/info"
	v := url.Values{}
	v.Set("last_trade_id", strconv.FormatInt(lastID, 10))
	res, err := c.get(path, v)
	if err != nil {
		return nil, errors.Wrapf(err, "GET %s request failed", path)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, errors.Wrapf(err, "GET %s body read failed", path)
		}
		return nil, errorWithStatus(errors.Errorf("GET %s failed.", path), res.StatusCode, string(b))
	}
	r := &InfoResponse{}
	if err := json.NewDecoder(res.Body).Decode(r); err != nil {
		return nil, errors.Wrapf(err, "GET %s body decode failed", path)
	}
	return r, nil
}

func (c *Client) AddOrder(ordertyp string, amount, price int64) (*Order, error) {
	path := "/orders"
	v := url.Values{}
	v.Set("type", ordertyp)
	v.Set("amount", strconv.FormatInt(amount, 10))
	v.Set("price", strconv.FormatInt(price, 10))
	res, err := c.post(path, v)
	if err != nil {
		return nil, errors.Wrapf(err, "POST %s request failed", path)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, errors.Wrapf(err, "POST %s body read failed", path)
		}
		return nil, errorWithStatus(errors.Errorf("POST %s failed.", path), res.StatusCode, string(b))
	}
	r := &OrderActionResponse{}
	if err := json.NewDecoder(res.Body).Decode(r); err != nil {
		return nil, errors.Wrapf(err, "POST %s body decode failed", path)
	}
	if r.ID == 0 {
		return nil, errors.Errorf("POST %s failed. id is not returned", path)
	}

	return &Order{
		ID:     r.ID,
		Amount: amount,
		Price:  price,
		Type:   ordertyp,
	}, nil
}

func (c *Client) GetOrders() ([]Order, error) {
	path := "/orders"
	res, err := c.get(path, url.Values{})
	if err != nil {
		return nil, errors.Wrapf(err, "GET %s request failed", path)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, errors.Wrapf(err, "GET %s body read failed", path)
		}
		return nil, errorWithStatus(errors.Errorf("GET %s failed.", path), res.StatusCode, string(b))
	}
	r := []Order{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, errors.Wrapf(err, "GET %s body decode failed", path)
	}
	return r, nil
}

func (c *Client) DeleteOrders(id int64) error {
	path := fmt.Sprintf("/order/%d", id)
	res, err := c.del(path, url.Values{})
	if err != nil {
		return errors.Wrapf(err, "DELETE %s request failed", path)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.Wrapf(err, "DELETE %s body read failed", path)
		}
		return errorWithStatus(errors.Errorf("DELETE %s failed.", path), res.StatusCode, string(b))
	}
	r := &OrderActionResponse{}
	if err := json.NewDecoder(res.Body).Decode(r); err != nil {
		return errors.Wrapf(err, "DELETE %s body decode failed", path)
	}
	if r.ID != id {
		return errors.Errorf("DELETE %s failed. id is not match requested value [got:%d, want:%d]", path, r.ID, id)
	}
	return nil
}
