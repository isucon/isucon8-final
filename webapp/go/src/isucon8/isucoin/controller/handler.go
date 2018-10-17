package controller

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"isucon8/isucoin/model"

	"github.com/gorilla/sessions"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

const (
	SessionName = "isucoin_session"
)

var BaseTime time.Time

type Handler struct {
	db    *sql.DB
	store sessions.Store
}

func NewHandler(db *sql.DB, store sessions.Store) *Handler {
	// ISUCON用初期データの基準時間です
	// この時間以降のデータはInitializeで削除されます
	BaseTime = time.Date(2018, 10, 16, 10, 0, 0, 0, time.Local)
	return &Handler{
		db:    db,
		store: store,
	}
}

func (h *Handler) Initialize(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	err := h.txScope(func(tx *sql.Tx) error {
		if err := model.InitBenchmark(tx); err != nil {
			return err
		}
		for _, k := range []string{
			model.BankEndpoint,
			model.BankAppid,
			model.LogEndpoint,
			model.LogAppid,
		} {
			if err := model.SetSetting(tx, k, r.FormValue(k)); err != nil {
				return errors.Wrapf(err, "set setting failed. %s", k)
			}
		}
		return nil
	})
	if err != nil {
		h.handleError(w, err, 500)
	} else {
		h.handleSuccess(w, struct{}{})
	}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	name := r.FormValue("name")
	bankID := r.FormValue("bank_id")
	password := r.FormValue("password")
	if name == "" || bankID == "" || password == "" {
		h.handleError(w, errors.New("all parameters are required"), 400)
		return
	}
	err := h.txScope(func(tx *sql.Tx) error {
		return model.UserSignup(tx, name, bankID, password)
	})
	switch {
	case err == model.ErrBankUserNotFound:
		// TODO: 失敗が多いときに403を返すBanの仕様に対応
		h.handleError(w, err, 404)
	case err == model.ErrBankUserConflict:
		h.handleError(w, err, 409)
	case err != nil:
		h.handleError(w, err, 500)
	default:
		h.handleSuccess(w, struct{}{})
	}
}

func (h *Handler) Signin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	bankID := r.FormValue("bank_id")
	password := r.FormValue("password")
	if bankID == "" || password == "" {
		h.handleError(w, errors.New("all parameters are required"), 400)
		return
	}
	user, err := model.UserLogin(h.db, bankID, password)
	switch {
	case err == model.ErrUserNotFound:
		// TODO: 失敗が多いときに403を返すBanの仕様に対応
		h.handleError(w, err, 404)
	case err != nil:
		h.handleError(w, err, 500)
	default:
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
		h.handleSuccess(w, user)
	}
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
	h.handleSuccess(w, struct{}{})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var (
		err         error
		lastTradeID int64
		lt          = time.Unix(0, 0)
		res         = make(map[string]interface{}, 10)
	)
	if _cursor := r.URL.Query().Get("cursor"); _cursor != "" {
		if lastTradeID, _ = strconv.ParseInt(_cursor, 10, 64); lastTradeID > 0 {
			trade, err := model.GetTradeByID(h.db, lastTradeID)
			if err != nil && err != sql.ErrNoRows {
				h.handleError(w, errors.Wrap(err, "getTradeByID failed"), 500)
				return
			}
			if trade != nil {
				lt = trade.CreatedAt
			}
		}
	}
	latestTrade, err := model.GetLatestTrade(h.db)
	if err != nil {
		h.handleError(w, errors.Wrap(err, "GetLatestTrade failed"), 500)
		return
	}
	res["cursor"] = latestTrade.ID
	user, _ := h.userByRequest(r)
	if user != nil {
		orders, err := model.GetOrdersByUserIDAndLastTradeId(h.db, user.ID, lastTradeID)
		if err != nil {
			h.handleError(w, err, 500)
			return
		}
		for _, order := range orders {
			if err = model.FetchOrderRelation(h.db, order); err != nil {
				h.handleError(w, err, 500)
				return
			}
		}
		res["traded_orders"] = orders
	}

	bySecTime := BaseTime.Add(-300 * time.Second)
	if lt.After(bySecTime) {
		bySecTime = time.Date(lt.Year(), lt.Month(), lt.Day(), lt.Hour(), lt.Minute(), lt.Second(), 0, lt.Location())
	}
	res["chart_by_sec"], err = model.GetCandlestickData(h.db, bySecTime, "%Y-%m-%d %H:%i:%s")
	if err != nil {
		h.handleError(w, errors.Wrap(err, "model.GetCandlestickData by sec"), 500)
		return
	}

	byMinTime := BaseTime.Add(-300 * time.Minute)
	if lt.After(byMinTime) {
		byMinTime = time.Date(lt.Year(), lt.Month(), lt.Day(), lt.Hour(), lt.Minute(), 0, 0, lt.Location())
	}
	res["chart_by_min"], err = model.GetCandlestickData(h.db, byMinTime, "%Y-%m-%d %H:%i:00")
	if err != nil {
		h.handleError(w, errors.Wrap(err, "model.GetCandlestickData by min"), 500)
		return
	}

	byHourTime := BaseTime.Add(-48 * time.Hour)
	if lt.After(byHourTime) {
		byHourTime = time.Date(lt.Year(), lt.Month(), lt.Day(), lt.Hour(), 0, 0, 0, lt.Location())
	}
	res["chart_by_hour"], err = model.GetCandlestickData(h.db, byHourTime, "%Y-%m-%d %H:00:00")
	if err != nil {
		h.handleError(w, errors.Wrap(err, "model.GetCandlestickData by hour"), 500)
		return
	}

	lowestSellOrder, err := model.GetLowestSellOrder(h.db)
	switch {
	case err == sql.ErrNoRows:
	case err != nil:
		h.handleError(w, errors.Wrap(err, "model.GetLowestSellOrder"), 500)
		return
	default:
		res["lowest_sell_price"] = lowestSellOrder.Price
	}

	highestBuyOrder, err := model.GetHighestBuyOrder(h.db)
	switch {
	case err == sql.ErrNoRows:
	case err != nil:
		h.handleError(w, errors.Wrap(err, "model.GetHighestBuyOrder"), 500)
		return
	default:
		res["highest_buy_price"] = highestBuyOrder.Price
	}
	// TODO: trueにするとシェアボタンが有効になるが、アクセスが増えてヤバイので一旦falseにしておく
	res["enable_share"] = false

	h.handleSuccess(w, res)
}

func (h *Handler) AddOrders(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user, err := h.userByRequest(r)
	if err != nil {
		h.handleError(w, err, 401)
		return
	}
	amount, _ := strconv.ParseInt(r.FormValue("amount"), 10, 64)
	price, _ := strconv.ParseInt(r.FormValue("price"), 10, 64)
	var order *model.Order
	err = h.txScope(func(tx *sql.Tx) (err error) {
		order, err = model.AddOrder(tx, r.FormValue("type"), user.ID, amount, price)
		return
	})
	switch {
	case err == model.ErrParameterInvalid || err == model.ErrCreditInsufficient:
		h.handleError(w, err, 400)
	case err != nil:
		h.handleError(w, err, 500)
	default:
		tradeChance, err := model.HasTradeChanceByOrder(h.db, order.ID)
		if err != nil {
			h.handleError(w, err, 500)
			return
		}
		if tradeChance {
			if err := model.RunTrade(h.db); err != nil {
				// トレードに失敗してもエラーにはしない
				log.Printf("runTrade err:%s", err)
			}
		}
		h.handleSuccess(w, map[string]interface{}{
			"id": order.ID,
		})
	}
}

func (h *Handler) GetOrders(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user, err := h.userByRequest(r)
	if err != nil {
		h.handleError(w, err, 401)
		return
	}
	orders, err := model.GetOrdersByUserID(h.db, user.ID)
	if err != nil {
		h.handleError(w, err, 500)
		return
	}
	for _, order := range orders {
		if err = model.FetchOrderRelation(h.db, order); err != nil {
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
	id, _ := strconv.ParseInt(p.ByName("id"), 10, 64)
	err = h.txScope(func(tx *sql.Tx) error {
		return model.DeleteOrder(tx, user.ID, id, "canceled")
	})
	switch {
	case err == model.ErrOrderNotFound || err == model.ErrOrderAlreadyClosed:
		h.handleError(w, err, 404)
	case err != nil:
		h.handleError(w, err, 500)
	default:
		h.handleSuccess(w, map[string]interface{}{
			"id": id,
		})
	}
}

func (h *Handler) CommonMiddleware(f http.Handler) http.Handler {
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
			user, err := model.GetUserByID(h.db, userID)
			switch {
			case err == sql.ErrNoRows:
				session.Values["user_id"] = 0
				session.Options = &sessions.Options{MaxAge: -1}
				if err = session.Save(r, w); err != nil {
					h.handleError(w, err, 500)
					return
				}
				h.handleError(w, errors.New("セッションが切断されました"), 404)
				return
			case err != nil:
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

func (h *Handler) userByRequest(r *http.Request) (*model.User, error) {
	v := r.Context().Value("user_id")
	if id, ok := v.(int64); ok {
		return model.GetUserByID(h.db, id)
	}
	return nil, errors.New("Not authenticated")
}

func (h *Handler) handleSuccess(w http.ResponseWriter, data interface{}) {
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[WARN] write response json failed. %s", err)
	}
}

func (h *Handler) handleError(w http.ResponseWriter, err error, code int) {
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

func (h *Handler) txScope(f func(*sql.Tx) error) (err error) {
	var tx *sql.Tx
	tx, err = h.db.Begin()
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
