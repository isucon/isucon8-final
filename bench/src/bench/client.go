package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"bench/urlcache"

	"github.com/pkg/errors"
	"golang.org/x/net/publicsuffix"
)

const (
	UserAgent     = "Isutrader/0.0.1"
	TradeTypeSell = "sell"
	TradeTypeBuy  = "buy"
)

var (
	ErrAlreadyRetired = errors.New("already retired client")
)

type ResponseWithElapsedTime struct {
	*http.Response
	ElapsedTime time.Duration
	Hash        string
}

type ErrElapsedTimeOverRetire struct {
	s string
}

func (e *ErrElapsedTimeOverRetire) Error() string {
	return e.s
}

type ErrorWithStatus struct {
	StatusCode int
	Body       string
	err        error
}

func errorWithStatus(err error, code int, body string) *ErrorWithStatus {
	body = strings.TrimSpace(body)
	if utf8.RuneCountInString(body) > 200 {
		if strings.Index(strings.ToLower(body), "<html") > -1 {
			body = "(html)"
		} else {
			body = string([]rune(body)[:200]) + "..."
		}
	}
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

func (o *Order) Removed() bool {
	return o.ClosedAt != nil && o.TradeID == 0
}

type CandlestickData struct {
	Time  time.Time `json:"time"`
	Open  int64     `json:"open"`
	Close int64     `json:"close"`
	High  int64     `json:"high"`
	Low   int64     `json:"low"`
}

type InfoResponse struct {
	Cursor          int64             `json:"cursor"`
	TradedOrders    []Order           `json:"traded_orders"`
	LowestSellPrice int64             `json:"lowest_sell_price"`
	HighestBuyPrice int64             `json:"highest_buy_price"`
	ChartBySec      []CandlestickData `json:"chart_by_sec"`
	ChartByMin      []CandlestickData `json:"chart_by_min"`
	ChartByHour     []CandlestickData `json:"chart_by_hour"`
	EnableShare     bool              `json:"enable_share"`
}

type OrderActionResponse struct {
	ID int64 `json:"id"`
}

type Client struct {
	base      *url.URL
	hc        *http.Client
	userID    int64
	bankid    string
	pass      string
	name      string
	cache     *urlcache.CacheStore
	retired   bool
	retireto  time.Duration
	topLoaded int32
}

func NewClient(base, bankid, name, password string, timeout, retire time.Duration) (*Client, error) {
	b, err := url.Parse(base)
	if err != nil {
		return nil, errors.Wrapf(err, "base url parse Failed.")
	}
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, errors.Wrapf(err, "cookiejar.New Failed.")
	}
	transport := &http.Transport{}
	hc := &http.Client{
		Jar:       jar,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: timeout,
	}
	return &Client{
		base:     b,
		hc:       hc,
		bankid:   bankid,
		name:     name,
		pass:     password,
		cache:    urlcache.NewCacheStore(),
		retireto: retire,
	}, nil
}

func (c *Client) IsRetired() bool {
	return c.retired
}

func (c *Client) UserID() int64 {
	return c.userID
}

func (c *Client) doRequest(ctx context.Context, req *http.Request) (*ResponseWithElapsedTime, error) {
	if c.retired {
		return nil, ErrAlreadyRetired
	}
	req.Header.Set("User-Agent", UserAgent)
	var reqbody []byte
	if req.Body != nil {
		var err error
		reqbody, err = ioutil.ReadAll(req.Body) // for retry
		if err != nil {
			return nil, errors.Wrapf(err, "reqbody read failed")
		}
	}
	start := time.Now()
	for {
		if reqbody != nil {
			req.Body = ioutil.NopCloser(bytes.NewBuffer(reqbody))
		}
		if ctx != nil {
			req = req.WithContext(ctx)
		}
		res, err := c.hc.Do(req)
		if err != nil {
			elapsedTime := time.Now().Sub(start)
			if e, ok := err.(*url.Error); ok {
				// log.Printf("[DEBUG] url.Error %#v", e)
				if e.Timeout() && c.retireto <= elapsedTime {
					c.retired = true
					return nil, &ErrElapsedTimeOverRetire{e.Error()}
				}
				switch e.Err {
				case context.Canceled, context.DeadlineExceeded:
					// こっちはcontext timeout
					return nil, e.Err
				}
			}
			log.Printf("[WARN] err: %s, [%.5f] req.len:%d", err, elapsedTime.Seconds(), req.ContentLength)
			if elapsedTime < c.retireto {
				continue
			}
			return nil, err
		}
		elapsedTime := time.Now().Sub(start)
		if c.retireto < elapsedTime {
			if err = res.Body.Close(); err != nil {
				log.Printf("[WARN] body close failed. %s", err)
			}
			c.retired = true
			return nil, &ErrElapsedTimeOverRetire{
				s: fmt.Sprintf("this user give up browsing because response time is too long. [%.5f s]", elapsedTime.Seconds()),
			}
		}
		if res.StatusCode < 500 {
			return &ResponseWithElapsedTime{res, elapsedTime, ""}, nil
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Printf("[INFO] retry status code: %d, read body failed: %s", res.StatusCode, err)
		} else {
			log.Printf("[INFO] retry status code: %d, body: %s", res.StatusCode, string(body))
		}
		time.Sleep(RetryInterval)
	}
}

func (c *Client) get(ctx context.Context, path string, val url.Values) (*ResponseWithElapsedTime, error) {
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
		// if cache.CanUseCache() {
		// 	return &ResponseWithElapsedTime{
		// 		Response: &http.Response{
		// 			StatusCode: http.StatusNotModified,
		// 			Body:       ioutil.NopCloser(&bytes.Buffer{}),
		// 		},
		// 		ElapsedTime: 0,
		// 	}, nil
		// }
		cache.ApplyRequest(req)
	}
	res, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == 200 {
		body := &bytes.Buffer{}
		if _, err = io.Copy(body, res.Body); err != nil {
			return nil, err
		}
		if cache, hash := urlcache.NewURLCache(res.Response, body); cache != nil {
			c.cache.Set(us, cache)
			res.Hash = hash
		}
		res.Body = ioutil.NopCloser(body)
	}
	return res, nil
}

func (c *Client) post(ctx context.Context, path string, val url.Values) (*ResponseWithElapsedTime, error) {
	u, err := c.base.Parse(path)
	if err != nil {
		return nil, errors.Wrap(err, "url parse failed")
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(val.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.doRequest(ctx, req)
}

func (c *Client) del(ctx context.Context, path string, val url.Values) (*ResponseWithElapsedTime, error) {
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
	return c.doRequest(ctx, req)
}

func (c *Client) Initialize(ctx context.Context, bankep, bankid, logep, logid string) error {
	v := url.Values{}
	v.Set("bank_endpoint", bankep)
	v.Set("bank_appid", bankid)
	v.Set("log_endpoint", logep)
	v.Set("log_appid", logid)
	res, err := c.post(ctx, "/initialize", v)
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

func (c *Client) Signup(ctx context.Context) error {
	v := url.Values{}
	v.Set("name", c.name)
	v.Set("bank_id", c.bankid)
	v.Set("password", c.pass)
	res, err := c.post(ctx, "/signup", v)
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

func (c *Client) Signin(ctx context.Context) error {
	v := url.Values{}
	v.Set("bank_id", c.bankid)
	v.Set("password", c.pass)
	res, err := c.post(ctx, "/signin", v)
	if err != nil {
		return errors.Wrap(err, "POST /signin request failed")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.Wrapf(err, "POST /signin body read failed")
		}
		return errorWithStatus(errors.Errorf("POST /signin failed."), res.StatusCode, string(b))
	}
	r := &User{}
	if err := json.NewDecoder(res.Body).Decode(r); err != nil {
		return errors.Wrapf(err, "POST /signin body decode failed")
	}
	if r.Name != c.name {
		return errors.Errorf("POST /signin returned different name [%s] my name is [%s]", r.Name, c.name)
	}
	if r.ID == 0 {
		return errors.Errorf("POST /signin returned zero id")
	}
	c.userID = r.ID
	return nil
}

func (c *Client) Signout(ctx context.Context) error {
	res, err := c.post(ctx, "/signout", url.Values{})
	if err != nil {
		return errors.Wrap(err, "POST /signout request failed")
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "POST /signout body read failed")
	}
	if res.StatusCode == http.StatusOK {
		return nil
	}
	return errorWithStatus(errors.Errorf("POST /signout failed."), res.StatusCode, string(b))
}

func (c *Client) Top(ctx context.Context) error {
	loaded := atomic.AddInt32(&c.topLoaded, 1)
	for _, sf := range StaticFiles {
		err := func(sf *StaticFile) error {
			res, err := c.get(ctx, sf.Path, url.Values{})
			if err != nil {
				return errors.Wrapf(err, "GET %s request failed", sf.Path)
			}
			defer res.Body.Close()
			b, err := ioutil.ReadAll(res.Body)
			if err != nil {
				return errors.Wrapf(err, "GET %s body read failed", sf.Path)
			}
			if res.StatusCode == 200 {
				// HashさえチェックすればSizeチェックはいらないはず?
				// if res.ContentLength != sf.Size {
				// 	return errors.Wrapf(err, "GET %s content length is not match. got:%d, want:%d", sf.Path, res.ContentLength, sf.Size)
				// }
				if res.Hash != sf.Hash {
					return errors.Wrapf(err, "GET %s content is modified.", sf.Path)
				}
				return nil
			} else if loaded > 1 && res.StatusCode == http.StatusNotModified {
				return nil
			}
			return errorWithStatus(errors.Errorf("GET %s failed.", sf.Path), res.StatusCode, string(b))
		}(sf)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Info(ctx context.Context, cursor int64) (*InfoResponse, error) {
	path := "/info"
	v := url.Values{}
	v.Set("cursor", strconv.FormatInt(cursor, 10))
	//log.Printf("[DEBUG] GET /info?cursor=%d [user:%d]", cursor, c.UserID())
	res, err := c.get(ctx, path, v)
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
	// 古いのだけで最新がないのはあり得る
	// TODO チャートデータのテスト
	// slen, mlen, hlen := len(r.ChartBySec), len(r.ChartByMin), len(r.ChartByHour)
	// if slen < mlen || mlen < hlen {
	// 	return nil, errors.Errorf("GET %s chart length is broken?", path)
	// }
	if r.Cursor == 0 {
		return nil, errors.Errorf("GET %s cursor is zero", path)
	}
	if r.TradedOrders != nil && len(r.TradedOrders) > 0 {
		if err := c.testMyOrder(path, r.TradedOrders); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (c *Client) AddOrder(ctx context.Context, ordertype string, amount, price int64) (*Order, error) {
	path := "/orders"
	v := url.Values{}
	v.Set("type", ordertype)
	v.Set("amount", strconv.FormatInt(amount, 10))
	v.Set("price", strconv.FormatInt(price, 10))
	//log.Printf("[DEBUG] POST /orders [user:%d]", c.UserID())
	res, err := c.post(ctx, path, v)
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
		Type:   ordertype,
	}, nil
}

func (c *Client) GetOrders(ctx context.Context) ([]Order, error) {
	path := "/orders"
	res, err := c.get(ctx, path, url.Values{})
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
	orders := []Order{}
	if err := json.NewDecoder(res.Body).Decode(&orders); err != nil {
		return nil, errors.Wrapf(err, "GET %s body decode failed", path)
	}
	if err := c.testMyOrder(path, orders); err != nil {
		return nil, err
	}
	return orders, nil
}

func (c *Client) DeleteOrders(ctx context.Context, id int64) error {
	path := fmt.Sprintf("/order/%d", id)
	//log.Printf("[DEBUG] DELETE %s [user:%d]", path, c.UserID())
	res, err := c.del(ctx, path, url.Values{})
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

func (c *Client) testMyOrder(path string, orders []Order) error {
	var tc time.Time
	for _, order := range orders {
		if order.UserID != c.userID {
			return errors.Errorf("GET %s returned not my order [id:%d, user_id:%d]", path, order.ID, c.UserID)
		}
		if order.User == nil {
			return errors.Errorf("GET %s returned not filled user [id:%d, user_id:%d]", path, order.ID, c.UserID)
		}
		if order.User.Name != c.name {
			return errors.Errorf("GET %s returned filled user.name is not my name [id:%d, user_id:%d]", path, order.ID, c.UserID)
		}
		if order.TradeID != 0 && order.Trade == nil {
			return errors.Errorf("GET %s returned not filled trade [id:%d, user_id:%d]", path, order.ID, c.UserID)
		}
		if order.CreatedAt.Before(tc) {
			return errors.Errorf("GET %s sort order is must be created_at desc", path)
		}
		tc = order.CreatedAt
	}
	return nil
}
