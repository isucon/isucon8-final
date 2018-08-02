package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

func main() {
	var (
		port   = flag.Int("port", 5516, "log app ranning port")
		dbhost = flag.String("dbhost", "127.0.0.1", "database host")
		dbport = flag.Int("dbport", 3306, "database port")
		dbuser = flag.String("dbuser", "root", "database user")
		dbpass = flag.String("dbpass", "", "database pass")
		dbname = flag.String("dbname", "isulog", "database name")
	)

	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	dbup := *dbuser
	if *dbpass != "" {
		dbup += ":" + *dbpass
	}

	dsn := fmt.Sprintf("%s@tcp(%s:%d)/%s?parseTime=true&loc=Local&charset=utf8mb4", dbup, *dbhost, *dbport, *dbname)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("mysql connect failed. err: %s", err)
	}
	server := NewServer(db)

	log.Printf("[INFO] start server %s", addr)
	//log.Fatal(http.ListenAndServe(addr, server))
	log.Fatal(http.ListenAndServe(addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		server.ServeHTTP(w, r)
		elasped := time.Now().Sub(start)
		log.Printf("%s\t%s\t%s\t%.5f", start.Format("2006-01-02T15:04:05.000"), r.Method, r.URL.Path, elasped.Seconds())
	})))
}

func NewServer(db *sql.DB) *http.ServeMux {
	server := http.NewServeMux()

	h := &Handler{
		db:    db,
		guard: make(map[string]chan struct{}, 1000),
	}

	server.HandleFunc("/send", h.Send)
	server.HandleFunc("/send_bulk", h.SendBulk)

	// default 404
	server.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[INFO] request not found %s", r.URL.RawPath)
		Error(w, "Not found", 404)
	})
	return server
}

type badRequestErr struct {
	s string
}

func BadRequestErrorf(s string, args ...interface{}) error {
	return &badRequestErr{fmt.Sprintf(s, args...)}
}

func (e *badRequestErr) Error() string {
	return e.s
}

func Error(w http.ResponseWriter, err string, code int) {
	http.Error(w, err, code)
}

func Success(w http.ResponseWriter) {
	fmt.Fprintln(w, "ok")
}

const (
	MaxBodySize   = 1024 * 1024 // 1MB
	Wait          = 25 * time.Millisecond
	MultiExec     = 2
	MySQLDatetime = "2006-01-02 15:04:05"
	LocationName  = "Asia/Tokyo"
)

type TagType int

const (
	TagSignup TagType = 1 + iota
	TagSignin
	TagSellOrder
	TagBuyOrder
	TagBuyError
	TagClose
	TagSellClose
	TagBuyClose
)

type Log struct {
	Tag  string          `json:"tag"`
	Time int64           `json:"time"`
	Data json.RawMessage `json:"data"`
}

type BulkLog struct {
	AppID string `json:"app_id"`
	Logs  []Log  `json:"logs"`
}

type SoloLog struct {
	Log
	AppID string `json:"app_id"`
}

type Signup struct {
	Name   string `json:"name"`
	BankID string `json:"bank_id"`
	UserID int64  `json:"user_id"`
}

type Signin struct {
	UserID int64 `json:"user_id"`
}

type SellOrder struct {
	UserID int64 `json:"user_id"`
	SellID int64 `json:"sell_id"`
	Amount int64 `json:"amount"`
	Price  int64 `json:"price"`
}

type BuyOrder struct {
	UserID int64 `json:"user_id"`
	BuyID  int64 `json:"buy_id"`
	Amount int64 `json:"amount"`
	Price  int64 `json:"price"`
}

type BuyError struct {
	UserID int64  `json:"user_id"`
	Amount int64  `json:"amount"`
	Price  int64  `json:"price"`
	Error  string `json:"error"`
}

type Close struct {
	TradeID int64 `json:"trade_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

type SellClose struct {
	TradeID int64 `json:"trade_id"`
	UserID  int64 `json:"user_id"`
	SellID  int64 `json:"sell_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

type BuyClose struct {
	TradeID int64 `json:"trade_id"`
	UserID  int64 `json:"user_id"`
	BuyID   int64 `json:"buy_id"`
	Amount  int64 `json:"amount"`
	Price   int64 `json:"price"`
}

type Handler struct {
	db    *sql.DB
	guard map[string]chan struct{}
}

func (s *Handler) lock(appid string) func() {
	if _, ok := s.guard[appid]; !ok {
		s.guard[appid] = make(chan struct{}, MultiExec)
	}
	s.guard[appid] <- struct{}{}
	return func() {
		<-s.guard[appid]
	}
}

func (s *Handler) Send(w http.ResponseWriter, r *http.Request) {
	req := &SoloLog{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		Error(w, "can't parse body", http.StatusBadRequest)
		return
	}
	if req.AppID == "" {
		Error(w, "app_id is required", http.StatusBadRequest)
		return
	}
	unlock := s.lock(req.AppID)
	defer unlock()
	err := s.putLog(req.Log, req.AppID)
	if err != nil {
		time.Sleep(Wait)
		if _, ok := err.(*badRequestErr); ok {
			Error(w, err.Error(), http.StatusBadRequest)
		} else {
			log.Printf("[WARN] %s", err)
			Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}
	time.Sleep(Wait)
	Success(w)
}

func (s *Handler) SendBulk(w http.ResponseWriter, r *http.Request) {
	req := &BulkLog{}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodySize)).Decode(req); err != nil {
		Error(w, "can't parse body", http.StatusBadRequest)
		return
	}
	if req.AppID == "" {
		Error(w, "app_id is required", http.StatusBadRequest)
		return
	}
	unlock := s.lock(req.AppID)
	defer unlock()
	errors := make([]error, 0, len(req.Logs))
	for _, l := range req.Logs {
		err := s.putLog(l, req.AppID)
		switch err {
		case nil:
		default:
			log.Printf("[WARN] %s", err)
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		Error(w, "internal server error", http.StatusInternalServerError)
	} else {
		time.Sleep(Wait)
		Success(w)
	}
}

func (s *Handler) putLog(l Log, appID string) error {
	if len(l.Data) == 0 {
		return BadRequestErrorf("%s data is required", l.Tag)
	}
	if l.Time < time.Now().Unix()-3600 {
		return BadRequestErrorf("%d time is too old", l.Time)
	}
	lt := time.Unix(l.Time, 0)
	var userID, tradeID int64
	var tag TagType
	// benchmarkerでどこまで見るかで各caseでinsertでも良い
	switch l.Tag {
	case "signup":
		tag = TagSignup
		data := &Signup{}
		if err := json.Unmarshal(l.Data, data); err != nil {
			return errors.Wrapf(err, "%s parse data failed", l.Tag)
		}
		if data.Name == "" {
			return BadRequestErrorf("%s data.name is required", l.Tag)
		}
		if data.BankID == "" {
			return BadRequestErrorf("%s data.bank_id is required", l.Tag)
		}
		if data.UserID == 0 {
			return BadRequestErrorf("%s data.user_id is required", l.Tag)
		}
		userID = data.UserID
	case "signin":
		tag = TagSignin
		data := &Signin{}
		if err := json.Unmarshal(l.Data, data); err != nil {
			return errors.Wrapf(err, "%s parse data failed", l.Tag)
		}
		if data.UserID == 0 {
			return BadRequestErrorf("%s data.user_id is required", l.Tag)
		}
		userID = data.UserID
	case "sell.order":
		tag = TagSellOrder
		data := &SellOrder{}
		if err := json.Unmarshal(l.Data, data); err != nil {
			return errors.Wrap(err, "parse data failed")
		}
		if data.UserID == 0 {
			return BadRequestErrorf("%s data.user_id is required", l.Tag)
		}
		if data.SellID == 0 {
			return BadRequestErrorf("%s data.sell_id is required", l.Tag)
		}
		if data.Amount == 0 {
			return BadRequestErrorf("%s data.amount is required", l.Tag)
		}
		if data.Price == 0 {
			return BadRequestErrorf("%s data.price is required", l.Tag)
		}
		userID = data.UserID
	case "buy.order":
		tag = TagBuyOrder
		data := &BuyOrder{}
		if err := json.Unmarshal(l.Data, data); err != nil {
			return errors.Wrap(err, "parse data failed")
		}
		if data.UserID == 0 {
			return BadRequestErrorf("%s data.user_id is required", l.Tag)
		}
		if data.BuyID == 0 {
			return BadRequestErrorf("%s data.buy_id is required", l.Tag)
		}
		if data.Amount == 0 {
			return BadRequestErrorf("%s data.amount is required", l.Tag)
		}
		if data.Price == 0 {
			return BadRequestErrorf("%s data.price is required", l.Tag)
		}
		userID = data.UserID
	case "buy.error":
		tag = TagBuyError
		data := &BuyError{}
		if err := json.Unmarshal(l.Data, data); err != nil {
			return errors.Wrap(err, "parse data failed")
		}
		if data.UserID == 0 {
			return BadRequestErrorf("%s data.user_id is required", l.Tag)
		}
		if data.Error == "" {
			return BadRequestErrorf("%s data.error is required", l.Tag)
		}
		if data.Amount == 0 {
			return BadRequestErrorf("%s data.amount is required", l.Tag)
		}
		if data.Price == 0 {
			return BadRequestErrorf("%s data.price is required", l.Tag)
		}
		userID = data.UserID
	case "close":
		tag = TagClose
		data := &Close{}
		if err := json.Unmarshal(l.Data, data); err != nil {
			return errors.Wrap(err, "parse data failed")
		}
		if data.TradeID == 0 {
			return BadRequestErrorf("%s data.trade_id is required", l.Tag)
		}
		if data.Amount == 0 {
			return BadRequestErrorf("%s data.amount is required", l.Tag)
		}
		if data.Price == 0 {
			return BadRequestErrorf("%s data.price is required", l.Tag)
		}
		tradeID = data.TradeID
	case "sell.close":
		tag = TagSellClose
		data := &SellClose{}
		if err := json.Unmarshal(l.Data, data); err != nil {
			return errors.Wrap(err, "parse data failed")
		}
		if data.TradeID == 0 {
			return BadRequestErrorf("%s data.trade_id is required", l.Tag)
		}
		if data.UserID == 0 {
			return BadRequestErrorf("%s data.user_id is required", l.Tag)
		}
		if data.SellID == 0 {
			return BadRequestErrorf("%s data.sell_id is required", l.Tag)
		}
		if data.Amount == 0 {
			return BadRequestErrorf("%s data.amount is required", l.Tag)
		}
		if data.Price == 0 {
			return BadRequestErrorf("%s data.price is required", l.Tag)
		}
		tradeID = data.TradeID
		userID = data.UserID
	case "buy.close":
		tag = TagBuyClose
		data := &BuyClose{}
		if err := json.Unmarshal(l.Data, data); err != nil {
			return errors.Wrap(err, "parse data failed")
		}
		if data.TradeID == 0 {
			return BadRequestErrorf("%s data.trade_id is required", l.Tag)
		}
		if data.UserID == 0 {
			return BadRequestErrorf("%s data.user_id is required", l.Tag)
		}
		if data.BuyID == 0 {
			return BadRequestErrorf("%s data.buy_id is required", l.Tag)
		}
		if data.Amount == 0 {
			return BadRequestErrorf("%s data.amount is required", l.Tag)
		}
		if data.Price == 0 {
			return BadRequestErrorf("%s data.price is required", l.Tag)
		}
		tradeID = data.TradeID
		userID = data.UserID
	default:
		return BadRequestErrorf("%s unknown tag", l.Tag)
	}

	query := `INSERT INTO log (app_id, tag, time, user_id, trade_id, data) VALUES (?, ?, ?, ?, ?, ?)`
	if _, err := s.db.Exec(query, appID, int(tag), lt.Format(MySQLDatetime), userID, tradeID, string(l.Data)); err != nil {
		return errors.Wrap(err, "insert log failed")
	}
	return nil
}

func init() {
	var err error
	loc, err := time.LoadLocation(LocationName)
	if err != nil {
		log.Panicln(err)
	}
	time.Local = loc
}
