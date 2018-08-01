package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

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

type Session struct {
	User *User
}

type OKResp struct {
	OK    bool   `jon:"ok"`
	Error string `jon:"error,omitempty"`
}

func NewServer(db *sql.DB, store sessions.Store) http.Handler {
	server := http.NewServeMux()

	h := &Handler{
		db:    db,
		store: store,
	}

	server.HandleFunc("/initialize", h.Initialize)
	server.HandleFunc("/signup", h.Signup)
	server.HandleFunc("/signin", h.Signin)
	server.HandleFunc("/signout", h.Signout)
	server.HandleFunc("/mypage", h.MyPage)
	server.HandleFunc("/sell_orders", h.SellOrders)
	server.HandleFunc("/buy_orders", h.BuyOrders)
	server.HandleFunc("/trades", h.Trades)

	// default 404
	server.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[INFO] request not found %s", r.URL.RawPath)
		http.Error(w, "Not found", 404)
	})

	return h.common(server)
}

type Handler struct {
	db    *sql.DB
	store sessions.Store
}

func (h *Handler) Initialize(w http.ResponseWriter, r *http.Request) {
	err := h.txScorp(func(tx *sql.Tx) error {
		query := `INSERT INTO setting (name, val) VALUES (?, ?) ON DUPLICATE KEY UPDATE val = VALUES(val)`
		for _, k := range []string{
			BankEndpoint,
			BankAppid,
			LogEndpoint,
			LogAppid,
		} {
			if _, err := tx.Exec(query, k, r.FormValue(k)); err != nil {
				return err
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

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		bankID := r.FormValue("bank_id")
		password := r.FormValue("password")
		if name == "" || bankID == "" || password == "" {
			h.handleError(w, errors.New("all paramaters are required"), http.StatusBadRequest)
			return
		}
		isubank, err := h.newIsubank()
		if err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
		logger, err := h.newLogger()
		if err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
		// bankIDの検証
		if err = isubank.Check(bankID, 1); err != nil {
			h.handleError(w, err, http.StatusBadRequest)
			return
		}
		pass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
		if res, err := h.db.Exec(`INSERT INTO user (bank_id, name, password, created_at) VALUES (?, ?, ?, NOW())`, bankID, name, pass); err != nil {
			if mysqlError, ok := err.(*mysql.MySQLError); ok {
				if mysqlError.Number == 1062 {
					h.handleError(w, errors.New("bank_id already exists"), http.StatusBadRequest)
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
		http.Redirect(w, r, "/signin", http.StatusFound)
	} else {
		// TODO Signup form or error
	}
}

func (h *Handler) Signin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		bankID := r.FormValue("bank_id")
		password := r.FormValue("password")
		if bankID == "" || password == "" {
			h.handleError(w, errors.New("all paramaters are required"), http.StatusBadRequest)
			return
		}
		logger, err := h.newLogger()
		if err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}

		var userID int64
		pass := []byte{}
		if err := h.db.QueryRow(`SELECT id, password FROM user WHERE bank_id = ?`, bankID).Scan(&userID, &pass); err != nil {
			if err == sql.ErrNoRows {
				h.handleError(w, errors.New("bank_idまたはpasswordが間違っています"), http.StatusNotFound)
				return
			}
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
		if err := bcrypt.CompareHashAndPassword(pass, []byte(password)); err != nil {
			if err == bcrypt.ErrMismatchedHashAndPassword {
				h.handleError(w, errors.New("bank_idまたはpasswordが間違っています"), http.StatusNotFound)
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
		session.Values["user_id"] = userID
		if err = session.Save(r, w); err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
		logger.Send("signin", LogDataSignin{
			UserID: userID,
		})
		http.Redirect(w, r, "/mypage", http.StatusFound)
	} else {
		// TODO Signin form or error
	}
}

func (h *Handler) Signout(w http.ResponseWriter, r *http.Request) {
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
	http.Redirect(w, r, "/signin", http.StatusFound)
}

func (h *Handler) MyPage(w http.ResponseWriter, r *http.Request) {
	s, err := h.auth(r)
	if err != nil {
		h.handleError(w, err, http.StatusUnauthorized)
		return
	}
	_ = s
}

func (h *Handler) SellOrders(w http.ResponseWriter, r *http.Request) {
	s, err := h.auth(r)
	if err != nil {
		h.handleError(w, err, http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodPost {
		res := &OKResp{
			OK: true,
		}
		err := h.txScorp(func(tx *sql.Tx) error {
			if _, err := h.getUserByIDWithLock(tx, s.User.ID); err != nil {
				return err
			}
			logger, err := h.newLogger()
			if err != nil {
				return err
			}
			amount, err := formvalInt64(r, "amount")
			if err != nil {
				return err
			}
			price, err := formvalInt64(r, "price")
			if err != nil {
				return err
			}
			res, err := tx.Exec(`INSERT INTO sell_order (user_id, amount, price, created_at) VALUES (?, ?, ?, NOW())`, s.User.ID, amount, price)
			if err != nil {
				return errors.Wrap(err, "insert sell_order failed")
			}
			sellID, err := res.LastInsertId()
			if err != nil {
				return errors.Wrap(err, "get sell_id failed")
			}
			err = logger.Send("sell.order", LogDataSellOrder{
				SellID: sellID,
				UserID: s.User.ID,
				Amount: amount,
				Price:  price,
			})
			if err != nil {
				return errors.Wrap(err, "send log failed")
			}
			return nil
		})
		if err != nil {
			res.OK = false
			res.Error = err.Error() // TODO message
		} else {
			h.runTrade()
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(res)
	} else {
		orders, err := h.getOrdersByUserID("sell_order", s.User.ID, ListLimit)
		if err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(orders)
	}
}

func (h *Handler) BuyOrders(w http.ResponseWriter, r *http.Request) {
	s, err := h.auth(r)
	if err != nil {
		h.handleError(w, err, http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodPost {
		res := &OKResp{
			OK: true,
		}
		err := h.txScorp(func(tx *sql.Tx) error {
			if _, err := h.getUserByIDWithLock(tx, s.User.ID); err != nil {
				return err
			}
			logger, err := h.newLogger()
			if err != nil {
				return err
			}
			isubank, err := h.newIsubank()
			if err != nil {
				return err
			}
			amount, err := formvalInt64(r, "amount")
			if err != nil {
				return err
			}
			price, err := formvalInt64(r, "price")
			if err != nil {
				return err
			}
			totalPrice := price * amount
			if err = isubank.Check(s.User.BankID, totalPrice); err != nil {
				logger.Send("buy.error", LogDataBuyError{
					Error:  err.Error(),
					UserID: s.User.ID,
					Amount: amount,
					Price:  price,
				})
				if err == ErrCreditInsufficient {
					return errors.New("銀行残高が足りません")
				}
				return errors.Wrap(err, "isubank check failed")
			}
			res, err := tx.Exec(`INSERT INTO buy_order (user_id, amount, price, created_at) VALUES (?, ?, ?, NOW())`, s.User.ID, amount, price)
			if err != nil {
				return errors.Wrap(err, "insert buy_order failed")
			}
			buyID, err := res.LastInsertId()
			if err != nil {
				return errors.Wrap(err, "get buy_id failed")
			}
			err = logger.Send("buy.order", LogDataBuyOrder{
				BuyID:  buyID,
				UserID: s.User.ID,
				Amount: amount,
				Price:  price,
			})
			if err != nil {
				return errors.Wrap(err, "send log failed")
			}
			return nil
		})
		if err != nil {
			res.OK = false
			res.Error = err.Error() // TODO message
		} else {
			h.runTrade()
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(res)
	} else {
		orders, err := h.getOrdersByUserID("buy_order", s.User.ID, ListLimit)
		if err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(orders)
	}
}

func (h *Handler) Trades(w http.ResponseWriter, r *http.Request) {
	trades, err := h.getTrades(ListLimit)
	if err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(trades)
}

func (h *Handler) runTrade() {
	isubank, err := h.newIsubank()
	if err != nil {
		log.Printf("[WARN] isubank init failed. err:%s", err)
		return
	}
	logger, err := h.newLogger()
	if err != nil {
		log.Printf("[WARN] logger init failed. err:%s", err)
		return
	}
	errNoItem := errors.New("no item")
	tradeBySell := func(tx *sql.Tx) error {
		reserves := []int64{}
		now := time.Now()
		// 一番安い売り注文
		var id int64
		q := `SELECT id FROM sell_order WHERE closed_at IS NULL ORDER BY price ASC LIMIT 1`
		err := tx.QueryRow(q).Scan(&id)
		switch {
		case err == sql.ErrNoRows:
			return errNoItem
		case err != nil:
			return err
		}
		sell, err := h.getOrderByIDWithLock(tx, "sell_order", id)
		if err != nil {
			return err
		}
		if sell.ClosedAt != nil {
			// 成約済み
			return nil
		}
		sell.User, err = h.getUserByIDWithLock(tx, sell.UserID)
		if err != nil {
			return err
		}
		restAmount := sell.Amount
		// 買い注文
		q = `SELECT id FROM buy_order WHERE closed_at IS NULL AND price >= ? ORDER BY price DESC`
		rows, err := tx.Query(q, sell.Price)
		if err != nil {
			return err
		}
		buys := []*Order{}
		defer rows.Close()
		for rows.Next() {
			var orderID int64
			if err = rows.Scan(&orderID); err != nil {
				return err
			}
			buy, err := h.getOrderByIDWithLock(tx, "buy_order", orderID)
			if err != nil {
				return err
			}
			if buy.ClosedAt != nil {
				// 成約済み
				continue
			}
			if buy.Amount > restAmount {
				continue
			}
			buy.User, err = h.getUserByIDWithLock(tx, buy.UserID)
			if err != nil {
				return err
			}
			resID, err := isubank.Reserve(buy.User.BankID, -sell.Price*buy.Amount)
			if err != nil {
				if err == ErrCreditInsufficient {
					// 与信確保失敗
					if _, err = tx.Exec(`UPDATE buy_order SET closed_at = ? WHERE id = ?`, now, buy.ID); err != nil {
						return err
					}
					continue
				}
				return err
			}
			reserves = append(reserves, resID)
			buys = append(buys, buy)
			restAmount -= buy.Amount
			if restAmount == 0 {
				break
			}
		}
		if err = rows.Err(); err != nil {
			return err
		}
		if restAmount > 0 {
			if len(reserves) > 0 {
				if err = isubank.Cancel(reserves); err != nil {
					return err
				}
			}
			return errNoItem
		}
		resID, err := isubank.Reserve(sell.User.BankID, sell.Price*sell.Amount)
		if err != nil {
			return err
		}
		reserves = append(reserves, resID)
		res, err := tx.Exec(`INSERT INTO trade (amount, price, created_at) VALUES (?, ?, ?)`, sell.Amount, sell.Price, now)
		if err != nil {
			return err
		}
		tradeID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		logger.Send("close", LogDataClose{
			Price:   sell.Price,
			Amount:  sell.Amount,
			TradeID: tradeID,
		})
		for _, buy := range buys {
			if _, err = tx.Exec(`UPDATE buy_order SET trade_id = ? AND closed_at = ? WHERE id = ?`, tradeID, now, buy.ID); err != nil {
				return err
			}
			logger.Send("buy.close", LogDataBuyClose{
				BuyID:   buy.ID,
				Price:   sell.Price,
				Amount:  buy.Amount,
				UserID:  buy.UserID,
				TradeID: tradeID,
			})
		}
		if _, err = tx.Exec(`UPDATE sell_order SET trade_id = ? AND closed_at = ? WHERE id = ?`, tradeID, now, sell.ID); err != nil {
			return err
		}
		logger.Send("sell.close", LogDataSellClose{
			SellID:  sell.ID,
			Price:   sell.Price,
			Amount:  sell.Amount,
			UserID:  sell.UserID,
			TradeID: tradeID,
		})
		if err = isubank.Commit(reserves); err != nil {
			return err
		}
		return nil
	}

	tradeByBuy := func(tx *sql.Tx) error {
		reserves := []int64{}
		now := time.Now()
		// 一番高い買い注文
		var id int64
		q := `SELECT id FROM buy_order WHERE closed_at IS NULL ORDER BY price DESC LIMIT 1`
		err := tx.QueryRow(q).Scan(&id)
		switch {
		case err == sql.ErrNoRows:
			return errNoItem
		case err != nil:
			return errors.Wrap(err, "select buyorder target failed")
		}
		buy, err := h.getOrderByIDWithLock(tx, "buy_order", id)
		if err != nil {
			return errors.Wrap(err, "getOrderByIDWithLock failed")
		}
		if buy.ClosedAt != nil {
			return nil
		}
		buy.User, err = h.getUserByIDWithLock(tx, buy.UserID)
		if err != nil {
			return errors.Wrap(err, "getUserByIDWithLock failed")
		}
		resID, err := isubank.Reserve(buy.User.BankID, -buy.Price*buy.Amount)
		if err != nil {
			if err == ErrCreditInsufficient {
				// 与信確保失敗
				if _, err = tx.Exec(`UPDATE buy_order SET closed_at = ? WHERE id = ?`, now, buy.ID); err != nil {
					return errors.Wrap(err, "update to closed")
				}
			}
			return errors.Wrap(err, "reserve failed")
		}
		reserves = append(reserves, resID)
		restAmount := buy.Amount
		// 売り
		q = `SELECT id FROM sell_order WHERE closed_at IS NULL AND price <= ? ORDER BY price ASC`
		rows, err := tx.Query(q, buy.Price)
		if err != nil {
			return errors.Wrap(err, "select sell_order")
		}
		sells := []*Order{}
		defer rows.Close()
		for rows.Next() {
			var orderID int64
			if err = rows.Scan(&orderID); err != nil {
				return errors.Wrap(err, "scan sell_order row")
			}
			sell, err := h.getOrderByIDWithLock(tx, "sell_order", orderID)
			if err != nil {
				return errors.Wrap(err, "getOrderByIDWithLock sell_order")
			}
			if sell.ClosedAt != nil {
				continue
			}
			if sell.Amount > restAmount {
				continue
			}
			sell.User, err = h.getUserByIDWithLock(tx, sell.UserID)
			if err != nil {
				return errors.Wrap(err, "getUserByIDWithLock")
			}
			resID, err := isubank.Reserve(sell.User.BankID, buy.Price*sell.Amount)
			if err != nil {
				return errors.Wrap(err, "reserve failed")
			}
			reserves = append(reserves, resID)
			sells = append(sells, sell)
			restAmount -= sell.Amount
			if restAmount == 0 {
				break
			}
		}
		if err = rows.Err(); err != nil {
			return errors.Wrap(err, "rows error")
		}
		if restAmount > 0 {
			if len(reserves) > 0 {
				if err = isubank.Cancel(reserves); err != nil {
					return errors.Wrap(err, "cancel failed")
				}
			}
			return errNoItem
		}
		res, err := tx.Exec(`INSERT INTO trade (amount, price, created_at) VALUES (?, ?, ?)`, buy.Amount, buy.Price, now)
		if err != nil {
			return errors.Wrap(err, "insert trade")
		}
		tradeID, err := res.LastInsertId()
		if err != nil {
			return errors.Wrap(err, "last insert id trade")
		}
		logger.Send("close", LogDataClose{
			Price:   buy.Price,
			Amount:  buy.Amount,
			TradeID: tradeID,
		})
		for _, sell := range sells {
			if _, err = tx.Exec(`UPDATE sell_order SET trade_id = ? AND closed_at = ? WHERE id = ?`, tradeID, now, sell.ID); err != nil {
				return errors.Wrap(err, "update sell_order")
			}
			logger.Send("sell.close", LogDataBuyClose{
				BuyID:   sell.ID,
				Price:   buy.Price,
				Amount:  sell.Amount,
				UserID:  sell.UserID,
				TradeID: tradeID,
			})
		}
		if _, err = tx.Exec(`UPDATE buy_order SET trade_id = ? AND closed_at = ? WHERE id = ?`, tradeID, now, buy.ID); err != nil {
			return errors.Wrap(err, "update buy_order")
		}
		logger.Send("buy.close", LogDataSellClose{
			SellID:  buy.ID,
			Price:   buy.Price,
			Amount:  buy.Amount,
			UserID:  buy.UserID,
			TradeID: tradeID,
		})
		if err = isubank.Commit(reserves); err != nil {
			return errors.Wrap(err, "commit")
		}
		return nil
	}

	err = nil
	for err == nil {
		err = h.txScorp(tradeBySell)
		if err != nil && err != errNoItem {
			log.Printf("[WARN] tradeBySell failed. err: %s", err)
		}
	}

	err = nil
	for err == nil {
		err = h.txScorp(tradeByBuy)
		if err != nil && err != errNoItem {
			log.Printf("[WARN] tradeByBuy failed. err: %s", err)
		}
	}
}

func (h *Handler) common(f http.Handler) http.Handler {
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
			user := &User{}
			ctx := r.Context()
			q := `SELECT id, bank_id, name, created_at FROM user WHERE id = ?`
			if err := h.db.QueryRow(q, userID).Scan(&user.ID, &user.BankID, &user.Name, &user.CreatedAt); err != nil {
				h.handleError(w, err, http.StatusInternalServerError)
				return
			}
			ctx = context.WithValue(ctx, "session", &Session{user})
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

func (h *Handler) txScorp(f func(*sql.Tx) error) (err error) {
	tx, err := h.db.Begin()
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

func (h *Handler) getUserByIDWithLock(tx *sql.Tx, id int64) (*User, error) {
	var user User
	q := `SELECT id, name, bank_id, created_at FROM user WHERE id = ? FOR UPDATE`
	err := h.db.QueryRow(q, id).Scan(&user.ID, &user.Name, &user.BankID, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (h *Handler) getUserByID(id int64) (*User, error) {
	var user User
	q := `SELECT id, name, bank_id, created_at FROM user WHERE id = ?`
	if err := h.db.QueryRow(q, id).Scan(&user.ID, &user.Name, &user.BankID, &user.CreatedAt); err != nil {
		return nil, err
	}
	return &user, nil
}

func (h *Handler) getTrades(limit int) ([]Trade, error) {
	trades := make([]Trade, 0, limit)
	q := `SELECT id, amount, price, created_at FROM trade ORDER BY created_at DESC LIMIT ?`
	rows, err := h.db.Query(q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var trade Trade
		if err = rows.Scan(&trade.ID, &trade.Amount, &trade.Price, &trade.CreatedAt); err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return trades, nil
}

func (h *Handler) getTradeByID(id int64) (*Trade, error) {
	var trade Trade
	q := `SELECT id, amount, price, created_at FROM trade WHERE id = ?`
	if err := h.db.QueryRow(q, id).Scan(&trade.ID, &trade.Amount, &trade.Price, &trade.CreatedAt); err != nil {
		return nil, err
	}
	return &trade, nil
}

func (h *Handler) getOrderByIDWithLock(tx *sql.Tx, table string, id int64) (*Order, error) {
	var order Order
	var closedAt mysql.NullTime
	var tradeID sql.NullInt64
	q := fmt.Sprintf(`SELECT id, user_id, amount, price, closed_at, trade_id, created_at FROM %s WHERE id = ? FOR UPDATE`, table)
	if err := tx.QueryRow(q, id).Scan(&order.ID, &order.UserID, &order.Amount, &order.Price, &closedAt, &tradeID, &order.CreatedAt); err != nil {
		return nil, err
	}
	if closedAt.Valid {
		order.ClosedAt = &closedAt.Time
	}
	if tradeID.Valid {
		order.TradeID = tradeID.Int64
	}
	return &order, nil
}

func (h *Handler) getOrdersByUserID(table string, userID int64, limit int) ([]Order, error) {
	orders := make([]Order, 0, limit)
	q := fmt.Sprintf(`SELECT id, user_id, amount, price, closed_at, trade_id, created_at FROM %s WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`, table)
	rows, err := h.db.Query(q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var closedAt mysql.NullTime
		var tradeID sql.NullInt64
		var order Order
		if err = rows.Scan(&order.ID, &order.UserID, &order.Amount, &order.Price, &closedAt, &tradeID, &order.CreatedAt); err != nil {
			return nil, err
		}
		if closedAt.Valid {
			order.ClosedAt = &closedAt.Time
		}
		if tradeID.Valid {
			order.TradeID = tradeID.Int64
		}
		order.User, err = h.getUserByID(order.UserID)
		if err != nil {
			return nil, err
		}
		if order.TradeID > 0 {
			order.Trade, err = h.getTradeByID(order.TradeID)
			if err != nil {
				return nil, err
			}
		}
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (h *Handler) newIsubank() (*Isubank, error) {
	ep, err := h.getSetting(BankEndpoint)
	if err != nil {
		return nil, err
	}
	id, err := h.getSetting(BankAppid)
	if err != nil {
		return nil, err
	}
	return NewIsubank(ep, id)
}

func (h *Handler) newLogger() (*Logger, error) {
	ep, err := h.getSetting(LogEndpoint)
	if err != nil {
		return nil, err
	}
	id, err := h.getSetting(LogAppid)
	if err != nil {
		return nil, err
	}
	return NewLogger(ep, id)
}

func (h *Handler) getSetting(k string) (v string, err error) {
	err = h.db.QueryRow(`SELECT val FROM setting WHERE name = ?`, k).Scan(&v)
	return
}

func formvalInt64(r *http.Request, key string) (int64, error) {
	v := r.FormValue(key)
	if v == "" {
		return 0, errors.Errorf("%s is required", key)
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		log.Printf("[INFO] can't parse to int64 key:%s val:%s err:%s", key, v, err)
		return 0, errors.Errorf("%s can't parse to int64")
	}
	return i, nil
}
