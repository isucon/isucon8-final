package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Songmu/strrand"
	"github.com/pkg/errors"
	"golang.org/x/net/publicsuffix"
)

var (
	RedirectAttemptedError = fmt.Errorf("redirect attempted")
	UserAgent              = "Isutrader/0.0.1"
	passwordGen            strrand.Generator
	nameGen                strrand.Generator
	createdAtUpper         = time.Now().Add(24 * time.Hour).Unix()
)

func init() {
	var err error
	if passwordGen, err = strrand.New().CreateGenerator(`[abcdefghjkmnpqrstuvwxyz23456789]{20}`); err != nil {
		panic(err)
	}
	if nameGen, err = strrand.New().CreateGenerator(`[あ-んア-ンa-zA-Z0-9]{16}`); err != nil {
		panic(err)
	}
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

type Client struct {
	base      *url.URL
	hc        *http.Client
	bankid    string
	pass      string
	name      string
	cache     *CacheStore
	postTime  map[string]time.Duration
	getTime   map[string]time.Duration
	postCount map[string]int64
	getCount  map[string]int64
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
		base:      b,
		hc:        hc,
		user:      nameGen.Generate(),
		pass:      passwordGen.Generate(),
		cache:     NewCacheStore(),
		postTime:  make(map[string]time.Duration, 20),
		getTime:   make(map[string]time.Duration, 20),
		postCount: make(map[string]int64, 20),
		getCount:  make(map[string]int64, 20),
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

func (c *Client) post(path string, ctype string, body io.Reader) (*http.Response, error) {
	u, err := c.base.Parse(path)
	if err != nil {
		return nil, errors.Wrap(err, "url parse failed")
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), body)
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Set("Content-Type", ctype)
	return c.doRequest(req)
}

func (c *Client) Signup() error {
	body := strings.NewReader(fmt.Sprintf(`{"name":"%s","bank_id":"%s","password":"%s"}`, c.name, c.bankid.c.pass))
	res, err := c.post("/signup", "application/json", body)
	if err != nil {
		return errors.Wrap(err, "POST /signup request failed")
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "POST /signup body read failed")
	}
	if res.StatusCode == 200 {
		return nil
	}
	return errors.Errorf("POST /signup failed. body: %s", string(b))
}

func (c *Client) Login() error {
	body := strings.NewReader(fmt.Sprintf(`{"user_id":"%s","password":"%s"}`, c.user, c.pass))
	res, err := c.post("/login", "application/json", body)
	if err != nil {
		return errors.Wrap(err, "POST /login request failed")
	}
	defer res.Body.Close()
	type Lres struct {
		Code      int    `json:"code"`
		SessionID string `json:"session_id"`
		Message   string `json:"message"`
	}
	var r Lres
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return errors.Wrap(err, "POST /login body decode failed")
	}
	if res.StatusCode == 200 {
		c.sessionID = r.SessionID
		return nil
	}
	return errors.Errorf("POST /login failed. json: %#v", r)
}

func (c *Client) AddScore(score int64) error {
	body := strings.NewReader(fmt.Sprintf(`{"session_id":"%s","score":%d}`, c.sessionID, score))
	res, err := c.post("/scores", "application/json", body)
	if err != nil {
		return errors.Wrap(err, "POST /scores request failed")
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "POST /scores body read failed")
	}
	if res.StatusCode == 200 {
		c.total += score
		if c.best < score {
			c.best = score
		}
		c.count++
		return nil
	}
	return errors.Errorf("POST /signup failed. body: %s", string(b))
}

func (c *Client) Static(path string, esize int64) error {
	res, err := c.get(path, url.Values{})
	if err != nil {
		return errors.Wrapf(err, "GET %s failed", path)
	}
	defer res.Body.Close()

	if res.StatusCode == 304 {
		return nil
	}
	if res.StatusCode != 200 {
		return errors.Errorf("GET %s status is %d", res.StatusCode)
	}

	size, err := io.Copy(ioutil.Discard, res.Body)
	if size < esize {
		return errors.Errorf("size is too small")
	}
	return nil
}

type Score struct {
	Rank      int    `json:"rank"`
	Name      string `json:"name"`
	Image     int    `json:"image"`
	Score     int64  `json:"score"`
	CreatedAt int64  `json:"created_at"`
}

type ScoreRes struct {
	Ranking []Score `json:"ranking"`
	Next    string  `json:"next"`
}

func (c *Client) Scores(path string, val url.Values) (*ScoreRes, error) {
	res, err := c.get(path, val)
	if err != nil {
		return nil, errors.Wrapf(err, "GET %s failed", path)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.Errorf("GET %s status code is %d", path, res.StatusCode)
	}
	r := &ScoreRes{}
	if err := json.NewDecoder(res.Body).Decode(r); err != nil {
		return nil, errors.Wrapf(err, "GET %s body decode failed", path)
	}
	return r, nil
}

func (c *Client) ScoresCallback(ctx context.Context, path string, count, pages int, cb func([]Score, int) error) error {
	statics := make([]string, 0, count)
	v := url.Values{}
	v.Set("count", strconv.Itoa(count))
	for i := 0; i < pages; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			res, err := c.Scores(path, v)
			if err != nil {
				return err
			}
			if len(res.Ranking) == 0 {
				return nil
			}
			images := make(map[int]struct{}, 10)
			// アイコンを取りに行く
			for _, rank := range res.Ranking {
				if _, ok := images[rank.Image]; !ok {
					statics = append(statics, fmt.Sprintf("/img/%d.jpg", rank.Image))
					images[rank.Image] = struct{}{}
				}
			}
			if err = c.RunStatics(ctx, statics); err != nil {
				return err
			}
			if err = cb(res.Ranking, i); err != nil {
				return err
			}
			if res.Next == "-1" || res.Next == "" {
				return nil
			}
			v.Set("next", res.Next)
		}
	}
	return nil
}

type UserScore struct {
	BestScore  int64 `json:"best_score"`
	BestRank   int64 `json:"best_rank"`
	TotalScore int64 `json:"total_score"`
	TotalRank  int64 `json:"total_rank"`
}

type User struct {
	ID     int64  `json:"id"`
	UserID string `json:"user_id"`
}

type UserInfoRes struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	User    User      `json:"user"`
	Score   UserScore `json:"score"`
}

func (c *Client) Info() (*UserInfoRes, error) {
	path := "/user"
	res, err := c.get(path, url.Values{"session_id": []string{c.sessionID}})
	if err != nil {
		return nil, errors.Wrapf(err, "GET %s failed", path)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.Errorf("GET %s status code is %d", path, res.StatusCode)
	}
	r := &UserInfoRes{}
	if err := json.NewDecoder(res.Body).Decode(r); err != nil {
		return nil, errors.Wrapf(err, "POST %s body decode failed", path)
	}
	return r, nil
}

func (c *Client) RunStatics(ctx context.Context, paths []string) error {
	// TODO htmlを読む
	for _, path := range paths {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := c.Static(path, 100); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Client) RunPlayGames(ctx context.Context, games int) error {
	if c.sessionID == "" {
		// sessionIDがあれば1度登録しているはず
		if err := c.Signup(); err != nil {
			return err
		}
	}
	if err := c.Login(); err != nil {
		return err
	}

	for i := 0; i < games; i++ {
		for _, task := range []func() error{
			func() error {
				score := rand.Int63n(10000) + 3000
				return c.AddScore(score)
			},
			func() error {
				info, err := c.Info()
				if err != nil {
					return err
				}
				if info.Score.BestScore != c.Best() {
					return errors.Errorf("best score が正しくありません")
				}
				if info.Score.TotalScore != c.Total() {
					return errors.Errorf("total score が正しくありません")
				}
				return nil
			},
		} {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if err := task(); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (c *Client) RunLatestScoresLoop(ctx context.Context, count, pages int) error {
	var ls *Score
	err := c.ScoresCallback(ctx, "/latest_scores", count, pages, func(s []Score, page int) error {
		if len(s) > count {
			return errors.Errorf("count パラメータが動いていません")
		}
		for _, score := range s {
			if err := ScoreCheck(score); err != nil {
				return err
			}
			if ls != nil && score.CreatedAt < ls.CreatedAt {
				return errors.Errorf("createdAtの降順に並んでいません")
			}
			ls = &score
		}
		return nil
	})
	return errors.Wrapf(err, "GET /latest_scores のリクエストに失敗")
}

func (c *Client) RunRankingScoresLoop(ctx context.Context, count, pages int) error {
	ranking := func(path string) error {
		var ls *Score
		return c.ScoresCallback(ctx, path, count, pages, func(s []Score, page int) error {
			if len(s) > count {
				return errors.Errorf("count パラメータが動いていません")
			}
			for i, score := range s {
				if err := ScoreCheck(score); err != nil {
					return err
				}
				if i > 0 {
					switch {
					case score.Score > s[i-1].Score:
						return errors.Errorf("Scoreの降順に並んでいません %d > %d", score.Score, s[i-1].Score)
					case score.Score == s[i-1].Score:
						if score.Rank != s[i-1].Rank {
							return errors.Errorf("同一スコア同一順位になっていません")
						}
					default:
						if score.Rank <= s[i-1].Rank {
							return errors.Errorf("Rankが正しくありません")
						}
					}
				} else {
					if ls != nil && ls.Score != score.Score && score.Rank < page*count+1 {
						return errors.Errorf("Rankが正しくありません")
					}
					if ls == nil && score.Rank != 1 {
						return errors.Errorf("Rankが正しくありません")
					}
				}
				ls = &score
			}
			return nil
		})
	}
	if err := ranking("/best_scores"); err != nil {
		return errors.Wrapf(err, "GET /best_scores のリクエストに失敗")
	}
	if err := ranking("/total_scores"); err != nil {
		return errors.Wrapf(err, "GET /total_scores のリクエストに失敗")
	}
	return nil
}

func ScoreCheck(s Score) error {
	if s.Score < 1 {
		return errors.Errorf("score の値が1以下")
	}
	if s.Image < 1 || 10 < s.Image {
		return errors.Errorf("image の値が範囲外")
	}
	if s.Image < 1 || 10 < s.Image {
		return errors.Errorf("image の値が範囲外")
	}
	if s.Name == "" {
		return errors.Errorf("Name が未設定")
	}
	if s.CreatedAt < 1523458800 || createdAtUpper < s.CreatedAt {
		return errors.Errorf("CreatedAt の範囲が不正")
	}
	return nil
}
