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

type Session struct {
	User *User
}

// errors

var (
	errClosedOrder  = errors.New("closed order")
	errNoOrder      = errors.New("no order")
	errPriceUnmatch = errors.New("price unmatch")
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
		h.handleError(w, err, http.StatusInternalServerError)
	} else {
		fmt.Fprintln(w, "ok")
	}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	name := r.FormValue("name")
	bankID := r.FormValue("bank_id")
	password := r.FormValue("password")
	if name == "" || bankID == "" || password == "" {
		h.handleError(w, errors.New("all paramaters are required"), http.StatusBadRequest)
		return
	}
	isubank, err := newIsubank(h.db)
	if err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}
	logger, err := newLogger(h.db)
	if err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}
	// bankIDの検証
	if err = isubank.Check(bankID, 0); err != nil {
		h.handleError(w, err, http.StatusNotFound)
		return
	}
	pass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}
	if res, err := h.db.Exec(`INSERT INTO user (bank_id, name, password, created_at) VALUES (?, ?, ?, NOW(6))`, bankID, name, pass); err != nil {
		if mysqlError, ok := err.(*mysql.MySQLError); ok {
			if mysqlError.Number == 1062 {
				h.handleError(w, errors.New("bank_id conflict"), http.StatusConflict)
				return
			}
		}
		h.handleError(w, err, http.StatusInternalServerError)
		return
	} else {
		userID, _ := res.LastInsertId()
		logger.Send("signup", LogDataSignup{
			BankID: bankID,
			UserID: userID,
			Name:   name,
		})
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintln(w, "{}")
}

func (h *Handler) Signin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	bankID := r.FormValue("bank_id")
	password := r.FormValue("password")
	if bankID == "" || password == "" {
		h.handleError(w, errors.New("all paramaters are required"), http.StatusBadRequest)
		return
	}
	logger, err := newLogger(h.db)
	if err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}

	user, err := getUserByBankID(h.db, bankID)
	if err != nil {
		if err == sql.ErrNoRows {
			h.handleError(w, errors.New("bank_id or password is not match"), http.StatusNotFound)
			return
		}
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			h.handleError(w, errors.New("bank_id or password is not match"), http.StatusNotFound)
			return
		}
		h.handleError(w, err, http.StatusBadRequest)
		return
	}
	session, err := h.store.Get(r, SessionName)
	if err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}
	session.Values["user_id"] = user.ID
	if err = session.Save(r, w); err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}
	logger.Send("signin", LogDataSignin{
		UserID: user.ID,
	})
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintln(w, "{}")
}

func (h *Handler) Signout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, err := h.store.Get(r, SessionName)
	if err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}
	session.Values["user_id"] = 0
	session.Options = &sessions.Options{MaxAge: -1}
	if err = session.Save(r, w); err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintln(w, "{}")
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	s, _ := h.auth(r)
	var lastTradeID int64
	if _lastInsertID := r.URL.Query().Get("last_trade_id"); _lastInsertID != "" {
		var err error
		lastTradeID, err = strconv.ParseInt(_lastInsertID, 10, 64)
		if err != nil {
			h.handleError(w, errors.Wrap(err, "last_trade_id parse failed"), http.StatusBadRequest)
			return
		}
	}
	trades, err := getTrades(h.db, lastTradeID)
	if err != nil {
		h.handleError(w, errors.Wrap(err, "getTrades failed"), http.StatusInternalServerError)
		return
	}
	lowestSellOrder, err := getLowestSellOrder(h.db)
	switch {
	case err == sql.ErrNoRows:
		lowestSellOrder = &Order{Price: 0}
	case err != nil:
		h.handleError(w, errors.Wrap(err, "find lowest sell order failed"), http.StatusInternalServerError)
		return
	}
	highestBuyOrder, err := getHighestBuyOrder(h.db)
	switch {
	case err == sql.ErrNoRows:
		highestBuyOrder = &Order{Price: 0}
	case err != nil:
		h.handleError(w, errors.Wrap(err, "find highest buy order failed"), http.StatusInternalServerError)
		return
	}
	res := map[string]interface{}{
		"trades":            trades,
		"lowest_sell_price": lowestSellOrder.Price,
		"highest_buy_price": highestBuyOrder.Price,
	}
	if s != nil {
		tradeIDs := make([]int64, len(trades))
		for i, trade := range trades {
			tradeIDs[i] = trade.ID
		}
		orders, err := getOrdersByUserIDAndTradeIds(h.db, s.User.ID, tradeIDs)
		if err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
		for _, order := range orders {
			if err = fetchOrderRelation(h.db, order); err != nil {
				h.handleError(w, err, http.StatusInternalServerError)
				return
			}
		}
		res["traded_orders"] = orders
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(res)
}

func (h *Handler) AddOrders(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	s, err := h.auth(r)
	if err != nil {
		h.handleError(w, err, http.StatusUnauthorized)
		return
	}

	var id int64
	err = txScorp(h.db, func(tx *sql.Tx) error {
		if _, err := getUserByIDWithLock(tx, s.User.ID); err != nil {
			return errors.Wrapf(err, "getUserByIDWithLock failed. id:%d", s.User.ID)
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
			if err = isubank.Check(s.User.BankID, totalPrice); err != nil {
				logger.Send("buy.error", LogDataBuyError{
					Error:  err.Error(),
					UserID: s.User.ID,
					Amount: amount,
					Price:  price,
				})
				if err == ErrCreditInsufficient {
					return errcode("銀行残高が足りません", 400)
				}
				return errors.Wrap(err, "isubank check failed")
			}
		case OrderTypeSell:
			// 売却のときは残高チェックは不要
			// TODO 椅子の保有チェック
		default:
			return errcode("type must be sell or buy", 400)
		}
		res, err := tx.Exec(`INSERT INTO orders (type, user_id, amount, price, created_at) VALUES (?, ?, ?, ?, NOW(6))`, ot, s.User.ID, amount, price)
		if err != nil {
			return errors.Wrap(err, "insert order failed")
		}
		id, err = res.LastInsertId()
		if err != nil {
			return errors.Wrap(err, "get order_id failed")
		}
		tag := ot + ".order"
		err = logger.Send(tag, LogDataOrder{
			OrderID: id,
			UserID:  s.User.ID,
			Amount:  amount,
			Price:   price,
		})
		if err != nil {
			return errors.Wrap(err, "send log failed")
		}
		return nil
	})
	if err != nil {
		if e, ok := err.(*errWithCode); ok {
			h.handleError(w, e.Err, e.StatusCode)
			return
		}
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}
	if err := runTrade(h.db); err != nil {
		// トレードに失敗してもエラーにはしない
		log.Printf("runTrade err:%s", err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, `{"id":%d}`, id)
}

func (h *Handler) GetOrders(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	s, err := h.auth(r)
	if err != nil {
		h.handleError(w, err, http.StatusUnauthorized)
		return
	}
	orders, err := getOrdersByUserID(h.db, s.User.ID)
	if err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}
	for _, order := range orders {
		if err = fetchOrderRelation(h.db, order); err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(orders)
}

func (h *Handler) DeleteOrders(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	s, err := h.auth(r)
	if err != nil {
		h.handleError(w, err, http.StatusUnauthorized)
		return
	}
	var id int64
	err = txScorp(h.db, func(tx *sql.Tx) error {
		if _, err := getUserByIDWithLock(tx, s.User.ID); err != nil {
			return errors.Wrapf(err, "getUserByIDWithLock failed. id:%d", s.User.ID)
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
		if order.UserID != s.User.ID {
			return errcodeWrap(errors.New("not found"), 404)
		}
		if order.ClosedAt != nil {
			return errcodeWrap(errors.New("already closed"), 404)
		}
		if _, err = tx.Exec(`UPDATE orders SET closed_at = ? WHERE id = ?`, time.Now(), order.ID); err != nil {
			return errors.Wrap(err, "update orders for cancel")
		}
		tag := order.Type + ".delete"
		logger.Send(tag, LogDataOrderDelete{
			OrderID: id,
			Reason:  "canceled",
		})
		return nil
	})

	if err != nil {
		if e, ok := err.(*errWithCode); ok {
			h.handleError(w, e.Err, e.StatusCode)
			return
		}
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, `{"id":%d}`, id)
}

func (h *Handler) commonHandler(f http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				h.handleError(w, err, http.StatusBadRequest)
				return
			}
		}
		session, err := h.store.Get(r, SessionName)
		if err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
		if _userID, ok := session.Values["user_id"]; ok {
			userID := _userID.(int64)
			user, err := getUserByID(h.db, userID)
			if err != nil {
				h.handleError(w, err, http.StatusInternalServerError)
				return
			}
			ctx := context.WithValue(r.Context(), "session", &Session{user})
			f.ServeHTTP(w, r.WithContext(ctx))
		} else {
			f.ServeHTTP(w, r)
		}
	})
}

func (h *Handler) auth(r *http.Request) (*Session, error) {
	v := r.Context().Value("session")
	if s, ok := v.(*Session); ok {
		return s, nil
	}
	return nil, errors.New("Not authenticate")
}

func (h *Handler) handleError(w http.ResponseWriter, err error, code int) {
	log.Printf("[WARN] err: %s", err.Error())
	// TODO Error Message
	http.Error(w, err.Error(), code)
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

func getTrades(d QueryExecuter, lastID int64) ([]*Trade, error) {
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

func fetchOrderRelation(d QueryExecuter, order *Order) error {
	var err error
	order.User, err = getUserByID(d, order.UserID)
	if err != nil {
		return errors.Wrapf(err, "getOrderByID failed. id")
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
			tag := order.Type + "..delete"
			logger.Send(tag, LogDataOrderDelete{
				OrderID: id,
				Reason:  "reserve_failed",
			})
			return 0, err
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
	res, err := tx.Exec(`INSERT INTO trade (amount, price, created_at) VALUES (?, ?, ?)`, order.Amount, order.Price, time.Now())
	if err != nil {
		return errors.Wrap(err, "insert trade")
	}
	tradeID, err := res.LastInsertId()
	if err != nil {
		return errors.Wrap(err, "lastInsertID for trade")
	}
	logger.Send("trade", LogDataTrade{
		TradeID: tradeID,
		Price:   order.Price,
		Amount:  order.Amount,
	})
	for _, o := range append(targets, order) {
		if _, err = tx.Exec(`UPDATE orders SET trade_id = ?, closed_at = ? WHERE id = ?`, tradeID, time.Now(), o.ID); err != nil {
			return errors.Wrap(err, "update order for trade")
		}
		logger.Send(o.Type+".trade", LogDataOrderTrade{
			OrderID: o.ID,
			Price:   order.Price,
			Amount:  o.Amount,
			UserID:  o.UserID,
			TradeID: tradeID,
		})
	}
	if err = isubank.Commit(reserves); err != nil {
		return errors.Wrap(err, "commit")
	}
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

	var targetIDs []int64
	switch order.Type {
	case OrderTypeBuy:
		targetIDs, err = queryInt64(tx, `SELECT id FROM orders WHERE type = ? AND closed_at IS NULL AND price <= ? ORDER BY price ASC, id ASC`, OrderTypeSell, order.Price)
	case OrderTypeSell:
		targetIDs, err = queryInt64(tx, `SELECT id FROM orders WHERE type = ? AND closed_at IS NULL AND price >= ? ORDER BY price DESC, id ASC`, OrderTypeBuy, order.Price)
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
		isubank, err := newIsubank(tx)
		if err != nil {
			return errors.Wrap(err, "isubank init failed")
		}
		if len(reserves) > 0 {
			if err = isubank.Cancel(reserves); err != nil {
				return errors.Wrap(err, "isubank.Cancel")
			}
		}
		return errNoOrder
	}
	return commitReservedOrder(tx, order, targets, reserves)
}

func runTrade(db *sql.DB) error {
	for {
		lowestSellOrder, err := getLowestSellOrder(db)
		switch {
		case err == sql.ErrNoRows:
			return errNoOrder
		case err != nil:
			return errors.Wrap(err, "find lowest sell order failed")
		}
		highestBuyOrder, err := getHighestBuyOrder(db)
		switch {
		case err == sql.ErrNoRows:
			return errNoOrder
		case err != nil:
			return errors.Wrap(err, "find highest buy order failed")
		}

		if lowestSellOrder.Price > highestBuyOrder.Price {
			// 売値が買値よりも高い
			return errPriceUnmatch
		}

		for _, orderID := range []int64{lowestSellOrder.ID, highestBuyOrder.ID} {
			err = txScorp(db, func(tx *sql.Tx) error {
				return tryTrade(tx, orderID)
			})
			switch err {
			case nil:
				break
			case errNoOrder, errClosedOrder:
				err = nil
				continue
			default:
				return err
			}
		}
	}
}
