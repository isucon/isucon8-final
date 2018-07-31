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
	RedirectAttemptedError = fmt.Errorf("redirect attempted")
	UserAgent              = "Isutrader/0.0.1"
	createdAtUpper         = time.Now().Add(24 * time.Hour).Unix()
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
	UserID    int64      `json:"user_id"`
	Amount    int64      `json:"amount"`
	Price     int64      `json:"price"`
	ClosedAt  *time.Time `json:"closed_at"`
	TradeID   int64      `json:"trade_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	User      *User      `json:"user,omitempty"`
	Trade     *Trade     `json:"trade,omitempty"`
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
			return RedirectAttemptedError
		},
		Timeout: timout,
	}
	return &Client{
		base:  b,
		hc:    hc,
		name:  name,
		pass:  password,
		cache: NewCacheStore(),
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
	return &ResponseWithElapsedTime{res, elapsedTime}
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
			c.getCount[path]++
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

func (c *Client) post(path string, val url.Values) (*http.Response, error) {
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
	return errors.Errorf("POST /initialize failed. body: %s", string(b))
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
	if res.StatusCode == http.StatusFound {
		return nil
	}
	return errors.Errorf("POST /signup failed. body: %s", string(b))
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
	if res.StatusCode == http.StatusFound {
		return nil
	}
	return errors.Errorf("POST /signin failed. body: %s", string(b))
}

func (c *Client) Mypage() error {
	res, err := c.get("/mypage", url.Values{})
	if err != nil {
		return errors.Wrap(err, "GET /mypage request failed")
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "GET /mypage body read failed")
	}
	if res.StatusCode == http.StatusOK {
		return nil
	}
	return errors.Errorf("POST /mypage failed. body: %s", string(b))
}

func (c *Client) Trades() ([]Trade, error) {
	path := "/trades"
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
		return nil, errors.Errorf("GET %s status code is %d, body: %s", path, res.StatusCode, string(b))
	}
	r := []Trade{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return errors.Wrapf(err, "GET %s body decode failed", path)
	}
	if r.OK {
		return r, nil
	}
	return nil, errors.Errorf("POST %s failed. err:%s", path, r.Error)
}

func (c *Client) AddSellOrder(amount, price int64) error {
	return c.addOrder("/sell_orders", amount, price)
}

func (c *Client) AddBuyOrder(amount, price int64) error {
	return c.addOrder("/buy_orders", amount, price)
}

func (c *Client) SellOrders() ([]Order, error) {
	return c.myOrders("/sell_orders")
}

func (c *Client) BuyOrders() ([]Order, error) {
	return c.myOrders("/buy_orders")
}

func (c *Client) addOrder(path string, amount, price int64) error {
	v := url.Values{}
	v.Set("amount", strconv.FormatInt(amount, 10))
	v.Set("price", strconv.FormatInt(price, 10))
	res, err := c.post(path, v)
	if err != nil {
		return errors.Wrapf(err, "POST %s request failed", path)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.Wrapf(err, "POST %s body read failed", path)
		}
		return errors.Errorf("POST %s status code is %d, body: %s", path, res.StatusCode, string(b))
	}
	r := &StatusRes{}
	if err := json.NewDecoder(res.Body).Decode(r); err != nil {
		return errors.Wrapf(err, "POST %s body decode failed", path)
	}
	if r.OK {
		return nil
	}
	return errors.Errorf("POST %s failed. err:%s", path, r.Error)
}

func (c *Client) myOrders(path string) ([]Order, error) {
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
		return nil, errors.Errorf("GET %s status code is %d, body: %s", path, res.StatusCode, string(b))
	}
	r := []Order{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return errors.Wrapf(err, "GET %s body decode failed", path)
	}
	if r.OK {
		return r, nil
	}
	return nil, errors.Errorf("POST %s failed. err:%s", path, r.Error)
}
