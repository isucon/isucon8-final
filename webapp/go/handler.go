package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// structs

type User struct {
	ID        int64     `json:"id"`
	BankID    string    `json:"-"`
	Name      string    `json:"name"`
	Password  string    `json:"-"`
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

type CandlestickData struct {
	Time  time.Time `json:"time"`
	Open  int64     `json:"open"`
	Close int64     `json:"close"`
	High  int64     `json:"high"`
	Low   int64     `json:"low"`
}

// errors

var (
	errClosedOrder  = errors.New("closed order")
	errNoOrder      = errors.New("no order")
	errPriceUnmatch = errors.New("price unmatch")
	empty           = struct{}{}
)

type errWithCode struct {
	StatusCode int
	Err        error
}

func (e *errWithCode) Error() string {
	return e.Err.Error()
}

func errcodeWrap(err error, code int) error {
	if err == nil {
		return nil
	}
	return &errWithCode{
		StatusCode: code,
		Err:        err,
	}
}

func errcode(err string, code int) error {
	return errcodeWrap(errors.New(err), code)
}

func NewServer(db *sql.DB, store sessions.Store, publicdir string) http.Handler {

	h := &Handler{
		db:    db,
		store: store,
	}

	router := httprouter.New()
	router.POST("/initialize", h.Initialize)
	router.POST("/signup", h.Signup)
	router.POST("/signin", h.Signin)
	router.POST("/signout", h.Signout)
	router.GET("/info", h.Info)
	router.POST("/orders", h.AddOrders)
	router.GET("/orders", h.GetOrders)
	router.DELETE("/order/:id", h.DeleteOrders)
	router.NotFound = http.FileServer(http.Dir(publicdir))

	return h.commonHandler(router)
}

type Handler struct {
	db    *sql.DB
	store sessions.Store
}

func (h *Handler) Initialize(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	err := txScorp(h.db, func(tx *sql.Tx) error {
		query := `INSERT INTO setting (name, val) VALUES (?, ?) ON DUPLICATE KEY UPDATE val = VALUES(val)`
		for _, k := range []string{
			BankEndpoint,
			BankAppid,
			LogEndpoint,
			LogAppid,
		} {
			if _, err := tx.Exec(query, k, r.FormValue(k)); err != nil {
				return errors.Wrapf(err, "set setting failed. %s", k)
			}
		}
		for _, q := range []string{
			"DELETE FROM user",
			"DELETE FROM orders",
			"DELETE FROM trade",
		} {
			if _, err := tx.Exec(q); err != nil {
				return errors.Wrapf(err, "query failed. %s", q)
			}
		}
		return nil
	})
	if err != nil {
		h.handleError(w, err, 500)
	} else {
		h.handleSuccess(w, empty)
	}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	name := r.FormValue("name")
	bankID := r.FormValue("bank_id")
	password := r.FormValue("password")
	if name == "" || bankID == "" || password == "" {
		h.handleError(w, errors.New("all paramaters are required"), 400)
		return
	}
	isubank, err := newIsubank(h.db)
	if err != nil {
		h.handleError(w, err, 500)
		return
	}
	logger, err := newLogger(h.db)
	if err != nil {
		h.handleError(w, err, 500)
		return
	}
	// bankIDの検証
	if err = isubank.Check(bankID, 0); err != nil {
		h.handleError(w, err, 404)
		return
	}
	pass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		h.handleError(w, err, 500)
		return
	}
	if res, err := h.db.Exec(`INSERT INTO user (bank_id, name, password, created_at) VALUES (?, ?, ?, NOW(6))`, bankID, name, pass); err != nil {
		if mysqlError, ok := err.(*mysql.MySQLError); ok {
			if mysqlError.Number == 1062 {
				h.handleError(w, errors.New("bank_id conflict"), 409)
				return
			}
		}
		h.handleError(w, err, 500)
		return
	} else {
		userID, _ := res.LastInsertId()
		err := logger.Send("signup", map[string]interface{}{
			"bank_id": bankID,
			"user_id": userID,
			"name":    name,
		})
		if err != nil {
			log.Printf("[WARN] logger.Send failed. err:%s", err)
		}
	}
	h.handleSuccess(w, empty)
}

func (h *Handler) Signin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	bankID := r.FormValue("bank_id")
	password := r.FormValue("password")
	if bankID == "" || password == "" {
		h.handleError(w, errors.New("all paramaters are required"), 400)
		return
	}
	logger, err := newLogger(h.db)
	if err != nil {
		h.handleError(w, err, 500)
		return
	}

	user, err := getUserByBankID(h.db, bankID)
	if err != nil {
		if err == sql.ErrNoRows {
			h.handleError(w, errors.New("bank_id or password is not match"), 404)
			return
		}
		h.handleError(w, err, 500)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			h.handleError(w, errors.New("bank_id or password is not match"), 404)
			return
		}
		h.handleError(w, err, 400)
		return
	}
	session, err := h.store.Get(r, SessionName)
	if err != nil {
		h.handleError(w, err, 500)
		return
	}
	session.Values["user_id"] = user.ID
	if err = session.Save(r, w); err != nil {
		h.handleError(w, err, 500)
		return
	}
	err = logger.Send("signin", map[string]interface{}{
		"user_id": user.ID,
	})
	if err != nil {
		log.Printf("[WARN] logger.Send failed. err:%s", err)
	}
	h.handleSuccess(w, user)
}

func (h *Handler) Signout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, err := h.store.Get(r, SessionName)
	if err != nil {
		h.handleError(w, err, 500)
		return
	}
	session.Values["user_id"] = 0
	session.Options = &sessions.Options{MaxAge: -1}
	if err = session.Save(r, w); err != nil {
		h.handleError(w, err, 500)
		return
	}
	h.handleSuccess(w, empty)
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var (
		err         error
		lastTradeID int64
		lt          = time.Unix(0, 0)
		res         = make(map[string]interface{}, 10)
	)
	if _cursor := r.URL.Query().Get("cursor"); _cursor != "" {
		lastTradeID, err = strconv.ParseInt(_cursor, 10, 64)
		if err != nil {
			h.handleError(w, errors.Wrap(err, "cursor parse failed"), 400)
			return
		}
		trade, err := getTradeByID(h.db, lastTradeID)
		if err != nil && err != sql.ErrNoRows {
			h.handleError(w, errors.Wrap(err, "getTradeByID failed"), 500)
			return
		}
		if trade != nil {
			lt = trade.CreatedAt
		}
	}
	res["cursor"] = lastTradeID
	trades, err := getTradesByLastID(h.db, lastTradeID)
	if err != nil {
		h.handleError(w, errors.Wrap(err, "getTradesByLastID failed"), 500)
		return
	}
	user, _ := h.userByRequest(r)
	if l := len(trades); l > 0 {
		res["cursor"] = trades[l-1].ID
		if user != nil {
			tradeIDs := make([]int64, len(trades))
			for i, trade := range trades {
				tradeIDs[i] = trade.ID
			}
			orders, err := getOrdersByUserIDAndTradeIds(h.db, user.ID, tradeIDs)
			if err != nil {
				h.handleError(w, err, 500)
				return
			}
			for _, order := range orders {
				if err = fetchOrderRelation(h.db, order); err != nil {
					h.handleError(w, err, 500)
					return
				}
			}
			res["traded_orders"] = orders
		}
	}

	bySecTime := time.Date(lt.Year(), lt.Month(), lt.Day(), lt.Hour(), lt.Minute(), lt.Second(), 0, lt.Location())
	chartBySec, err := getCandlestickData(h.db, bySecTime, "%Y-%m-%d %H:%i:%s")
	if err != nil {
		h.handleError(w, errors.Wrap(err, "getCandlestickData by sec"), 500)
		return
	}
	res["chart_by_sec"] = chartBySec

	byMinTime := time.Date(lt.Year(), lt.Month(), lt.Day(), lt.Hour(), lt.Minute(), 0, 0, lt.Location())
	chartByMin, err := getCandlestickData(h.db, byMinTime, "%Y-%m-%d %H:%i:00")
	if err != nil {
		h.handleError(w, errors.Wrap(err, "getCandlestickData by min"), 500)
		return
	}
	res["chart_by_min"] = chartByMin

	byHourTime := time.Date(lt.Year(), lt.Month(), lt.Day(), lt.Hour(), 0, 0, 0, lt.Location())
	chartByHour, err := getCandlestickData(h.db, byHourTime, "%Y-%m-%d %H:00:00")
	if err != nil {
		h.handleError(w, errors.Wrap(err, "getCandlestickData by hour"), 500)
		return
	}
	res["chart_by_hour"] = chartByHour

	lowestSellOrder, err := getLowestSellOrder(h.db)
	switch {
	case err == sql.ErrNoRows:
	case err != nil:
		h.handleError(w, errors.Wrap(err, "getLowestSellOrder"), 500)
		return
	default:
		res["lowest_sell_price"] = lowestSellOrder.Price
	}

	highestBuyOrder, err := getHighestBuyOrder(h.db)
	switch {
	case err == sql.ErrNoRows:
	case err != nil:
		h.handleError(w, errors.Wrap(err, "getHighestBuyOrder"), 500)
		return
	default:
		res["highest_buy_price"] = highestBuyOrder.Price
	}

	h.handleSuccess(w, res)
}

func (h *Handler) AddOrders(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user, err := h.userByRequest(r)
	if err != nil {
		h.handleError(w, err, 401)
		return
	}

	var id int64
	err = txScorp(h.db, func(tx *sql.Tx) error {
		if _, err := getUserByIDWithLock(tx, user.ID); err != nil {
			return errors.Wrapf(err, "getUserByIDWithLock failed. id:%d", user.ID)
		}
		logger, err := newLogger(tx)
		if err != nil {
			return errors.Wrap(err, "newLogger failed")
		}
		isubank, err := newIsubank(tx)
		if err != nil {
			return errors.Wrap(err, "newIsubank failed")
		}
		amount, err := formvalInt64(r, "amount")
		if err != nil {
			return errcodeWrap(errors.Wrapf(err, "formvalInt64 failed. amount"), 400)
		}
		if amount <= 0 {
			return errcodeWrap(errors.Errorf("amount is must be greater 0. [%d]", amount), 400)
		}
		price, err := formvalInt64(r, "price")
		if err != nil {
			return errcodeWrap(errors.Wrapf(err, "formvalInt64 failed. price"), 400)
		}
		if price <= 0 {
			return errcodeWrap(errors.Errorf("price is must be greater 0. [%d]", price), 400)
		}
		ot := r.FormValue("type")
		switch ot {
		case OrderTypeBuy:
			totalPrice := price * amount
			if err = isubank.Check(user.BankID, totalPrice); err != nil {
				le := logger.Send("buy.error", map[string]interface{}{
					"error":   err.Error(),
					"user_id": user.ID,
					"amount":  amount,
					"price":   price,
				})
				if le != nil {
					log.Printf("[WARN] logger.Send failed. err:%s", le)
				}
				if err == ErrCreditInsufficient {
					return errcode("銀行残高が足りません", 400)
				}
				return errors.Wrap(err, "isubank check failed")
			}
		case OrderTypeSell:
			// TODO 椅子の保有チェック
		default:
			return errcode("type must be sell or buy", 400)
		}
		res, err := tx.Exec(`INSERT INTO orders (type, user_id, amount, price, created_at) VALUES (?, ?, ?, ?, NOW(6))`, ot, user.ID, amount, price)
		if err != nil {
			return errors.Wrap(err, "insert order failed")
		}
		id, err = res.LastInsertId()
		if err != nil {
			return errors.Wrap(err, "get order_id failed")
		}
		le := logger.Send(ot+".order", map[string]interface{}{
			"order_id": id,
			"user_id":  user.ID,
			"amount":   amount,
			"price":    price,
		})
		if le != nil {
			log.Printf("[WARN] logger.Send failed. err:%s", le)
		}
		return nil
	})
	if err != nil {
		h.handleError(w, err, 500)
		return
	}

	tradeChance, err := hasTradeChanceByOrder(h.db, id)
	if err != nil {
		h.handleError(w, err, 500)
		return
	}
	if tradeChance {
		if err := runTrade(h.db); err != nil {
			// トレードに失敗してもエラーにはしない
			log.Printf("runTrade err:%s", err)
		}
	}
	h.handleSuccess(w, map[string]interface{}{
		"id": id,
	})
}

func (h *Handler) GetOrders(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user, err := h.userByRequest(r)
	if err != nil {
		h.handleError(w, err, 401)
		return
	}
	orders, err := getOrdersByUserID(h.db, user.ID)
	if err != nil {
		h.handleError(w, err, 500)
		return
	}
	for _, order := range orders {
		if err = fetchOrderRelation(h.db, order); err != nil {
			h.handleError(w, err, 500)
			return
		}
	}
	h.handleSuccess(w, orders)
}

func (h *Handler) DeleteOrders(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	user, err := h.userByRequest(r)
	if err != nil {
		h.handleError(w, err, 401)
		return
	}
	var id int64
	err = txScorp(h.db, func(tx *sql.Tx) error {
		if _, err := getUserByIDWithLock(tx, user.ID); err != nil {
			return errors.Wrapf(err, "getUserByIDWithLock failed. id:%d", user.ID)
		}
		logger, err := newLogger(tx)
		if err != nil {
			return errors.Wrap(err, "newLogger failed")
		}
		_id := p.ByName("id")
		if _id == "" {
			return errcodeWrap(errors.Errorf("id is required"), 400)
		}
		id, err = strconv.ParseInt(_id, 10, 64)
		if err != nil {
			return errcodeWrap(errors.Wrap(err, "id parse failed"), 400)
		}
		order, err := getOrderByIDWithLock(tx, id)
		if err != nil {
			err = errors.Wrapf(err, "getOrderByIDWithLock failed. id")
			if err == sql.ErrNoRows {
				return errcodeWrap(err, 404)
			}
			return err
		}
		if order.UserID != user.ID {
			return errcodeWrap(errors.New("not found"), 404)
		}
		if order.ClosedAt != nil {
			return errcodeWrap(errors.New("already closed"), 404)
		}
		if _, err = tx.Exec(`UPDATE orders SET closed_at = ? WHERE id = ?`, time.Now(), order.ID); err != nil {
			return errors.Wrap(err, "update orders for cancel")
		}
		le := logger.Send(order.Type+".delete", map[string]interface{}{
			"order_id": id,
			"reason":   "canceled",
		})
		if le != nil {
			log.Printf("[WARN] logger.Send failed. err:%s", le)
		}
		return nil
	})

	if err != nil {
		h.handleError(w, err, 500)
		return
	}

	h.handleSuccess(w, map[string]interface{}{
		"id": id,
	})
}

func (h *Handler) commonHandler(f http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				h.handleError(w, err, 400)
				return
			}
		}
		session, err := h.store.Get(r, SessionName)
		if err != nil {
			h.handleError(w, err, 500)
			return
		}
		if _userID, ok := session.Values["user_id"]; ok {
			userID := _userID.(int64)
			user, err := getUserByID(h.db, userID)
			if err != nil {
				h.handleError(w, err, 500)
				return
			}
			ctx := context.WithValue(r.Context(), "user_id", user.ID)
			f.ServeHTTP(w, r.WithContext(ctx))
		} else {
			f.ServeHTTP(w, r)
		}
	})
}

func (h *Handler) userByRequest(r *http.Request) (*User, error) {
	v := r.Context().Value("user_id")
	if id, ok := v.(int64); ok {
		return getUserByID(h.db, id)
	}
	return nil, errors.New("Not userByRequestenticate")
}

func (h *Handler) handleSuccess(w http.ResponseWriter, data interface{}) {
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[WARN] write response json failed. %s", err)
	}
}

func (h *Handler) handleError(w http.ResponseWriter, err error, code int) {
	if e, ok := err.(*errWithCode); ok {
		code = e.StatusCode
		err = e.Err
	}
	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	log.Printf("[WARN] err: %s", err.Error())
	data := map[string]interface{}{
		"code": code,
		"err":  err.Error(),
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[WARN] write error response json failed. %s", err)
	}
}

// helpers

func formvalInt64(r *http.Request, key string) (int64, error) {
	v := r.FormValue(key)
	if v == "" {
		return 0, errors.Errorf("%s is required", key)
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		log.Printf("[INFO] can't parse to int64 key:%s val:%s err:%s", key, v, err)
		return 0, errors.Errorf("%s can't parse to int64", key)
	}
	return i, nil
}

func txScorp(db *sql.DB, f func(*sql.Tx) error) (err error) {
	var tx *sql.Tx
	tx, err = db.Begin()
	if err != nil {
		return errors.Wrap(err, "begin transaction failed")
	}
	defer func() {
		if e := recover(); e != nil {
			tx.Rollback()
			err = errors.Errorf("panic in transaction: %s", e)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	err = f(tx)
	return
}

// databases

const (
	userColumns   = "id,bank_id,name,password,created_at"
	ordersColumns = "id,type,user_id,amount,price,closed_at,trade_id,created_at"
	tradeColumns  = "id,amount,price,created_at"
)

type RowScanner interface {
	Scan(...interface{}) error
}

func scanUser(r RowScanner) (*User, error) {
	var v User
	if err := r.Scan(&v.ID, &v.BankID, &v.Name, &v.Password, &v.CreatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}

func scanTrade(r RowScanner) (*Trade, error) {
	var v Trade
	if err := r.Scan(&v.ID, &v.Amount, &v.Price, &v.CreatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}

func scanOrder(r RowScanner) (*Order, error) {
	var v Order
	var closedAt mysql.NullTime
	var tradeID sql.NullInt64
	if err := r.Scan(&v.ID, &v.Type, &v.UserID, &v.Amount, &v.Price, &closedAt, &tradeID, &v.CreatedAt); err != nil {
		return nil, err
	}
	if closedAt.Valid {
		v.ClosedAt = &closedAt.Time
	}
	if tradeID.Valid {
		v.TradeID = tradeID.Int64
	}
	return &v, nil
}

type QueryExecuter interface {
	Exec(string, ...interface{}) (sql.Result, error)
	QueryRow(string, ...interface{}) *sql.Row
	Query(string, ...interface{}) (*sql.Rows, error)
}

func getSettingValue(d QueryExecuter, k string) (v string, err error) {
	err = d.QueryRow(`SELECT val FROM setting WHERE name = ?`, k).Scan(&v)
	return
}

func newIsubank(d QueryExecuter) (*Isubank, error) {
	ep, err := getSettingValue(d, BankEndpoint)
	if err != nil {
		return nil, errors.Wrapf(err, "getSetting failed. %s", BankEndpoint)
	}
	id, err := getSettingValue(d, BankAppid)
	if err != nil {
		return nil, errors.Wrapf(err, "getSetting failed. %s", BankAppid)
	}
	return NewIsubank(ep, id)
}

func newLogger(d QueryExecuter) (*Logger, error) {
	ep, err := getSettingValue(d, LogEndpoint)
	if err != nil {
		return nil, errors.Wrapf(err, "getSetting failed. %s", LogEndpoint)
	}
	id, err := getSettingValue(d, LogAppid)
	if err != nil {
		return nil, errors.Wrapf(err, "getSetting failed. %s", LogAppid)
	}
	return NewLogger(ep, id)
}

func getUserByBankID(d QueryExecuter, bankID string) (*User, error) {
	query := fmt.Sprintf("SELECT %s FROM user WHERE bank_id = ?", userColumns)
	return scanUser(d.QueryRow(query, bankID))
}

func getUserByID(d QueryExecuter, id int64) (*User, error) {
	query := fmt.Sprintf("SELECT %s FROM user WHERE id = ?", userColumns)
	return scanUser(d.QueryRow(query, id))
}

func getUserByIDWithLock(tx *sql.Tx, id int64) (*User, error) {
	query := fmt.Sprintf("SELECT %s FROM user WHERE id = ? FOR UPDATE", userColumns)
	return scanUser(tx.QueryRow(query, id))
}

func getTradeByID(d QueryExecuter, id int64) (*Trade, error) {
	query := fmt.Sprintf("SELECT %s FROM trade WHERE id = ?", tradeColumns)
	return scanTrade(d.QueryRow(query, id))
}

func getTradesByLastID(d QueryExecuter, lastID int64) ([]*Trade, error) {
	query := fmt.Sprintf("SELECT %s FROM trade WHERE id > ? ORDER BY id ASC", tradeColumns)
	rows, err := d.Query(query, lastID)
	if err != nil {
		return nil, errors.Wrapf(err, "Query failed. query:%s, lastID:%d", query, lastID)
	}
	defer rows.Close()
	trades := []*Trade{}
	for rows.Next() {
		trade, err := scanTrade(rows)
		if err != nil {
			return nil, errors.Wrapf(err, "Scan failed.")
		}
		trades = append(trades, trade)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "rows.Err failed.")
	}
	return trades, nil
}

func getCandlestickData(d QueryExecuter, mt time.Time, tf string) ([]CandlestickData, error) {
	query := fmt.Sprintf(`
		SELECT m.t, a.price, b.price, m.h, m.l
		FROM (
			SELECT
				STR_TO_DATE(DATE_FORMAT(created_at, '%s'), '%s') AS t,
				MIN(id) AS min_id,
				MAX(id) AS max_id,
				MAX(price) AS h,
				MIN(price) AS l
			FROM trade
			WHERE created_at >= ?
			GROUP BY t
		) m
		JOIN trade a ON a.id = m.min_id
		JOIN trade b ON b.id = m.max_id
		ORDER BY m.t
	`, tf, "%Y-%m-%d %H:%i:%s")
	rows, err := d.Query(query, mt)
	if err != nil {
		return nil, errors.Wrapf(err, "Query failed. query:%s, starttime:%s", query, mt)
	}
	defer rows.Close()
	datas := []CandlestickData{}
	for rows.Next() {
		var cd CandlestickData
		if err = rows.Scan(&cd.Time, &cd.Open, &cd.Close, &cd.High, &cd.Low); err != nil {
			return nil, errors.Wrapf(err, "Scan failed.")
		}
		datas = append(datas, cd)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "rows.Err failed.")
	}
	return datas, nil
}

func queryInt64(d QueryExecuter, q string, args ...interface{}) ([]int64, error) {
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "query failed. sql:%s", q)
	}
	defer rows.Close()
	is := []int64{}
	for rows.Next() {
		var i int64
		if err := rows.Scan(&i); err != nil {
			return nil, errors.Wrap(err, "scan failed")
		}
		is = append(is, i)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Error")
	}
	return is, nil
}

func getOrdersByUserID(d QueryExecuter, userID int64) ([]*Order, error) {
	query := fmt.Sprintf(`SELECT %s FROM orders WHERE user_id = ? AND (closed_at IS NULL OR trade_id IS NOT NULL) ORDER BY id ASC`, ordersColumns)
	return queryOrders(d, query, userID)
}

func getOrdersByUserIDAndTradeIds(d QueryExecuter, userID int64, tradeIDs []int64) ([]*Order, error) {
	if len(tradeIDs) == 0 {
		tradeIDs = []int64{0}
	}
	win := strings.Repeat(",?", len(tradeIDs))
	win = win[1:]
	args := make([]interface{}, 0, len(tradeIDs)+1)
	args = append(args, userID)
	for _, id := range tradeIDs {
		args = append(args, id)
	}
	query := fmt.Sprintf(`SELECT %s FROM orders WHERE user_id = ? AND trade_id IN (%s) ORDER BY id ASC`, ordersColumns, win)
	return queryOrders(d, query, args...)
}

func queryOrders(d QueryExecuter, query string, args ...interface{}) ([]*Order, error) {
	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "Query failed. query:%s, args:% v", query, args)
	}
	defer rows.Close()
	orders := []*Order{}
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, errors.Wrapf(err, "Scan failed.")
		}
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "rows.Err failed.")
	}
	return orders, nil
}

func getOrderByID(d QueryExecuter, id int64) (*Order, error) {
	query := fmt.Sprintf("SELECT %s FROM orders WHERE id = ?", ordersColumns)
	return scanOrder(d.QueryRow(query, id))
}

func getOrderByIDWithLock(tx *sql.Tx, id int64) (*Order, error) {
	query := fmt.Sprintf("SELECT %s FROM orders WHERE id = ? FOR UPDATE", ordersColumns)
	return scanOrder(tx.QueryRow(query, id))
}

func getLowestSellOrder(d QueryExecuter) (*Order, error) {
	q := fmt.Sprintf("SELECT %s FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price ASC, id ASC LIMIT 1", ordersColumns)
	return scanOrder(d.QueryRow(q, OrderTypeSell))
}

func getHighestBuyOrder(d QueryExecuter) (*Order, error) {
	q := fmt.Sprintf("SELECT %s FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price DESC, id ASC LIMIT 1", ordersColumns)
	return scanOrder(d.QueryRow(q, OrderTypeBuy))
}

func hasTradeChanceByOrder(d QueryExecuter, orderID int64) (bool, error) {
	order, err := getOrderByID(d, orderID)
	if err != nil {
		return false, err
	}

	lowest, err := getLowestSellOrder(d)
	switch {
	case err == sql.ErrNoRows:
		return false, nil
	case err != nil:
		return false, errors.Wrap(err, "getLowestSellOrder")
	}

	highest, err := getHighestBuyOrder(d)
	switch {
	case err == sql.ErrNoRows:
		return false, nil
	case err != nil:
		return false, errors.Wrap(err, "getHighestBuyOrder")
	}

	switch order.Type {
	case OrderTypeBuy:
		if lowest.Price <= order.Price {
			return true, nil
		}
	case OrderTypeSell:
		if order.Price <= highest.Price {
			return true, nil
		}
	default:
		return false, errors.Errorf("other type [%s]", order.Type)
	}
	return false, nil
}

func fetchOrderRelation(d QueryExecuter, order *Order) error {
	var err error
	order.User, err = getUserByID(d, order.UserID)
	if err != nil {
		return errors.Wrapf(err, "getUserByID failed. id")
	}
	if order.TradeID > 0 {
		order.Trade, err = getTradeByID(d, order.TradeID)
		if err != nil {
			return errors.Wrapf(err, "getTradeByID failed. id")
		}
	}
	return nil
}

func getOpenOrderByID(tx *sql.Tx, id int64) (*Order, error) {
	order, err := getOrderByIDWithLock(tx, id)
	if err != nil {
		return nil, errors.Wrap(err, "getOrderByIDWithLock sell_order")
	}
	if order.ClosedAt != nil {
		return nil, errClosedOrder
	}
	order.User, err = getUserByIDWithLock(tx, order.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "getUserByIDWithLock sell user")
	}
	return order, nil
}

func reserveOrder(d QueryExecuter, order *Order, price int64) (int64, error) {
	isubank, err := newIsubank(d)
	if err != nil {
		return 0, errors.Wrap(err, "isubank init failed")
	}
	logger, err := newLogger(d)
	if err != nil {
		return 0, errors.Wrap(err, "logger init failed")
	}
	p := order.Amount * price
	if order.Type == OrderTypeBuy {
		p *= -1
	}

	id, err := isubank.Reserve(order.User.BankID, p)
	if err != nil {
		if err == ErrCreditInsufficient {
			// 与信確保失敗した場合はorderを破棄する
			if _, err = d.Exec(`UPDATE orders SET closed_at = ? WHERE id = ?`, time.Now(), order.ID); err != nil {
				return 0, errors.Wrap(err, "update buy_order for cancel")
			}
			le := logger.Send(order.Type+".delete", map[string]interface{}{
				"order_id": id,
				"reason":   "reserve_failed",
			})
			if le != nil {
				log.Printf("[WARN] logger.Send failed. err:%s", le)
			}
			return 0, ErrCreditInsufficient
		}
		return 0, errors.Wrap(err, "isubank.Reserve")
	}

	return id, nil
}

func commitReservedOrder(tx *sql.Tx, order *Order, targets []*Order, reserves []int64) error {
	isubank, err := newIsubank(tx)
	if err != nil {
		return errors.Wrap(err, "isubank init failed")
	}
	logger, err := newLogger(tx)
	if err != nil {
		return errors.Wrap(err, "logger init failed")
	}
	defer func() {
		if len(reserves) > 0 {
			if err = isubank.Cancel(reserves); err != nil {
				log.Printf("[WARN] isubank cancel failed. err:%s", err)
			}
		}
	}()
	res, err := tx.Exec(`INSERT INTO trade (amount, price, created_at) VALUES (?, ?, ?)`, order.Amount, order.Price, time.Now())
	if err != nil {
		return errors.Wrap(err, "insert trade")
	}
	tradeID, err := res.LastInsertId()
	if err != nil {
		return errors.Wrap(err, "lastInsertID for trade")
	}
	le := logger.Send("trade", map[string]interface{}{
		"trade_id": tradeID,
		"price":    order.Price,
		"amount":   order.Amount,
	})
	if le != nil {
		log.Printf("[WARN] logger.Send failed. err:%s", le)
	}
	for _, o := range append(targets, order) {
		if _, err = tx.Exec(`UPDATE orders SET trade_id = ?, closed_at = ? WHERE id = ?`, tradeID, time.Now(), o.ID); err != nil {
			return errors.Wrap(err, "update order for trade")
		}
		le := logger.Send(o.Type+".trade", map[string]interface{}{
			"order_id": o.ID,
			"price":    order.Price,
			"amount":   o.Amount,
			"user_id":  o.UserID,
			"trade_id": tradeID,
		})
		if le != nil {
			log.Printf("[WARN] logger.Send failed. err:%s", le)
		}
	}
	if err = isubank.Commit(reserves); err != nil {
		return errors.Wrap(err, "commit")
	}
	reserves = reserves[:0]
	return nil
}

func tryTrade(tx *sql.Tx, orderID int64) error {
	order, err := getOpenOrderByID(tx, orderID)
	if err != nil {
		return err
	}

	restAmount := order.Amount
	unitPrice := order.Price
	reserves := make([]int64, 1, order.Amount+1)
	targets := make([]*Order, 0, order.Amount)

	reserves[0], err = reserveOrder(tx, order, unitPrice)
	if err != nil {
		return err
	}
	defer func() {
		if len(reserves) > 0 {
			isubank, err := newIsubank(tx)
			if err != nil {
				log.Printf("[WARN] isubank init failed. err:%s", err)
				return
			}
			if err = isubank.Cancel(reserves); err != nil {
				log.Printf("[WARN] isubank cancel failed. err:%s", err)
			}
		}
	}()

	var targetIDs []int64
	switch order.Type {
	case OrderTypeBuy:
		targetIDs, err = queryInt64(tx, `SELECT id FROM orders WHERE type = ? AND closed_at IS NULL AND price <= ? ORDER BY price ASC, created_at ASC, id ASC`, OrderTypeSell, order.Price)
	case OrderTypeSell:
		targetIDs, err = queryInt64(tx, `SELECT id FROM orders WHERE type = ? AND closed_at IS NULL AND price >= ? ORDER BY price DESC, created_at ASC, id ASC`, OrderTypeBuy, order.Price)
	}
	if err != nil {
		return errors.Wrap(err, "find target orders")
	}
	if len(targetIDs) == 0 {
		return errNoOrder
	}

	for _, tid := range targetIDs {
		to, err := getOpenOrderByID(tx, tid)
		if err != nil {
			if err == errClosedOrder {
				continue
			}
			return errors.Wrap(err, "getOrderByIDWithLock  buy_order")
		}
		if to.Amount > restAmount {
			continue
		}
		rid, err := reserveOrder(tx, to, unitPrice)
		if err != nil {
			if err == ErrCreditInsufficient {
				continue
			}
			return err
		}
		reserves = append(reserves, rid)
		targets = append(targets, to)
		restAmount -= to.Amount
		if restAmount == 0 {
			break
		}
	}
	if restAmount > 0 {
		return errNoOrder
	}
	// cancelをしたいので
	r := make([]int64, len(reserves))
	copy(r, reserves)
	reserves = reserves[:0]
	return commitReservedOrder(tx, order, targets, r)
}

func runTrade(db *sql.DB) error {
	lowestSellOrder, err := getLowestSellOrder(db)
	switch {
	case err == sql.ErrNoRows:
		// 売り注文が無いため成立しない
		return nil
	case err != nil:
		return errors.Wrap(err, "getLowestSellOrder")
	}

	highestBuyOrder, err := getHighestBuyOrder(db)
	switch {
	case err == sql.ErrNoRows:
		// 買い注文が無いため成立しない
		return nil
	case err != nil:
		return errors.Wrap(err, "getHighestBuyOrder")
	}

	if lowestSellOrder.Price > highestBuyOrder.Price {
		// 最安の売値が最高の買値よりも高いため成立しない
		return nil
	}

	candidates := make([]int64, 0, 2)
	if lowestSellOrder.Amount > highestBuyOrder.Amount {
		candidates = append(candidates, lowestSellOrder.ID, highestBuyOrder.ID)
	} else {
		candidates = append(candidates, highestBuyOrder.ID, lowestSellOrder.ID)
	}

	for _, orderID := range candidates {
		err := func() error {
			tx, err := db.Begin()
			if err != nil {
				return errors.Wrap(err, "begin transaction failed")
			}
			err = tryTrade(tx, orderID)
			switch err {
			case nil, errNoOrder, errClosedOrder, ErrCreditInsufficient:
				tx.Commit()
			default:
				tx.Rollback()
			}
			return err
		}()
		switch err {
		case nil:
			// トレード成立したため次の取引を行う
			return runTrade(db)
		case errNoOrder, errClosedOrder:
			// 注文個数の多い方で成立しなかったので少ない方で試す
			continue
		default:
			return err
		}
	}
	// 個数のが不足していて不成立
	return nil
}
