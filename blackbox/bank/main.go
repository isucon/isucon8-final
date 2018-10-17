package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	//	"encoding/json"
	//	"flag"
	//	"fmt"
	//	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

const (
	ResOK        = `{}`
	ResError     = `{"error":"%s"}`
	LocationName = "Asia/Tokyo"
	AxLog        = false
	AppIDCtxKey  = "appid"
)

var cacheBankID = make(map[string]int64, 1000)
var cacheBankIDMutex sync.RWMutex

func main() {
	var (
		port   = flag.Int("port", 5515, "bank app running port")
		dbhost = flag.String("dbhost", "127.0.0.1", "database host")
		dbport = flag.Int("dbport", 3306, "database port")
		dbuser = flag.String("dbuser", "root", "database user")
		dbpass = flag.String("dbpass", "", "database pass")
		dbname = flag.String("dbname", "isubank", "database name")
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
	if AxLog {
		log.Fatal(http.ListenAndServe(addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			server.ServeHTTP(w, r)
			elapsed := time.Now().Sub(start)
			log.Printf("%s\t%s\t%s\t%.5f", start.Format("2006-01-02T15:04:05.000"), r.Method, r.URL.Path, elapsed.Seconds())
		})))
	} else {
		log.Fatal(http.ListenAndServe(addr, server))
	}
}

func NewServer(db *sql.DB) http.Handler {
	server := http.NewServeMux()

	h := &Handler{db}
	server.HandleFunc("/register", h.Register)
	server.HandleFunc("/add_credit", h.AddCredit)
	server.HandleFunc("/credit", h.GetCredit)
	server.HandleFunc("/initialize", h.Initialize)
	server.HandleFunc("/check", sleepHandle(h.Check, 50*time.Millisecond))
	server.HandleFunc("/reserve", sleepHandle(h.Reserve, 70*time.Millisecond))
	server.HandleFunc("/commit", sleepHandle(h.Commit, 300*time.Millisecond))
	server.HandleFunc("/cancel", sleepHandle(h.Cancel, 80*time.Millisecond))

	// default 404
	server.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[INFO] request not found %s", r.URL.RawPath)
		Error(w, "Not found", 404)
	})

	return authHandler(server)
}

func authHandler(f http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		as := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(as) == 2 {
			switch as[0] {
			case "app_id", "Bearer":
				ctx = context.WithValue(ctx, AppIDCtxKey, as[1])
			}
		}
		f.ServeHTTP(w, r.WithContext(ctx))
	})
}

func sleepHandle(f http.HandlerFunc, sleep time.Duration) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(sleep)
		f.ServeHTTP(w, r)
	})
}

func appID(r *http.Request) (string, error) {
	v := r.Context().Value(AppIDCtxKey)
	if v == nil {
		return "", errors.Errorf("Authorization failed (no header)")
	}
	id, ok := v.(string)
	if !ok {
		return "", errors.Errorf("Authorization failed (cast appid)")
	}
	return id, nil
}

var (
	CreditIsInsufficient      = errors.New("credit is insufficient")
	ReserveIsExpires          = errors.New("reserve is already expired")
	ReserveIsAlreadyCommitted = errors.New("reserve is already committed")
)

func Error(w http.ResponseWriter, err string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	fmt.Fprintln(w, fmt.Sprintf(ResError, err))
}

func Success(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintln(w, ResOK)
}

type Handler struct {
	db *sql.DB
}

// Register は POST /register を処理
// ユーザーを作成します。本来はきっととても複雑な処理なのでしょうが誰でも簡単に一瞬で作れるのが特徴です
func (s *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type ReqParam struct {
		BankID string `json:"bank_id"`
	}
	req := &ReqParam{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		Error(w, "can't parse body", http.StatusBadRequest)
		return
	}
	if req.BankID == "" {
		Error(w, "bank_id is required", http.StatusBadRequest)
		return
	}
	if _, err := s.db.Exec(`INSERT INTO user (bank_id, created_at) VALUES (?, NOW(6))`, req.BankID); err != nil {
		if mysqlError, ok := err.(*mysql.MySQLError); ok {
			if mysqlError.Number == 1062 {
				Error(w, "bank_id already exists", http.StatusBadRequest)
				return
			}
		}
		log.Printf("[WARN] insert user failed. err: %s", err)
		Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	Success(w)
}

// AddCredit は POST /add_credit を処理
// とても簡単に残高を増やすことができます。本当の銀行ならこんなAPIは無いと思いますが...
func (s *Handler) AddCredit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type ReqPram struct {
		BankID string `json:"bank_id"`
		Price  int64  `json:"price"`
	}
	req := &ReqPram{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		Error(w, "can't parse body", http.StatusBadRequest)
		return
	}
	if req.Price <= 0 {
		Error(w, "price must be upper than 0", http.StatusBadRequest)
		return
	}
	userID := s.filterBankID(w, req.BankID)
	if userID <= 0 {
		return
	}
	err := s.txScope(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`SELECT id FROM user WHERE id = ? LIMIT 1 FOR UPDATE`, userID); err != nil {
			return errors.Wrap(err, "select lock failed")
		}
		return s.modifyCredit(tx, userID, req.Price, "by add credit API")
	})
	if err != nil {
		log.Printf("[WARN] addCredit failed. err: %s", err)
		Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	Success(w)
}

// GetCredit は Get /credit を処理
// ユーザーの残高をこっそり確認できます
func (s *Handler) GetCredit(w http.ResponseWriter, r *http.Request) {
	bankID := r.URL.Query().Get("bank_id")
	userID := s.filterBankID(w, bankID)
	if userID <= 0 {
		return
	}
	var credit int64
	if err := s.db.QueryRow(`SELECT credit FROM user WHERE id = ? LIMIT 1`, userID).Scan(&credit); err != nil {
		Error(w, fmt.Sprintf("select credit failed. err:%s", err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintln(w, fmt.Sprintf(`{"credit":%d}`, credit))
}

// Check は POST /check を処理
// 確定済み要求金額を保有しているかどうかを確認します
func (s *Handler) Check(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, err := appID(r)
	if err != nil {
		Error(w, err.Error(), http.StatusForbidden)
		return
	}
	type ReqPram struct {
		BankID string `json:"bank_id"`
		Price  int64  `json:"price"`
	}
	req := &ReqPram{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		Error(w, "can't parse body", http.StatusBadRequest)
		return
	}
	if req.Price < 0 {
		Error(w, "price must be upper 0", http.StatusBadRequest)
		return
	}
	userID := s.filterBankID(w, req.BankID)
	if userID <= 0 {
		return
	}
	if req.Price == 0 {
		Success(w)
		return
	}
	err = s.txScope(func(tx *sql.Tx) error {
		var credit int64
		if err := tx.QueryRow(`SELECT credit FROM user WHERE id = ? LIMIT 1 FOR UPDATE`, userID).Scan(&credit); err != nil {
			return errors.Wrap(err, "select credit failed")
		}
		if credit < req.Price {
			return CreditIsInsufficient
		}
		return nil
	})
	switch {
	case err == CreditIsInsufficient:
		Error(w, "credit is insufficient", http.StatusBadRequest)
	case err != nil:
		log.Printf("[WARN] check failed. err: %s", err)
		Error(w, "internal server error", http.StatusInternalServerError)
	default:
		Success(w)
	}
}

// Reserve は POST /reserve を処理
// 複数の取引をまとめるために1分間以内のCommitを保証します
func (s *Handler) Reserve(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	appid, err := appID(r)
	if err != nil {
		Error(w, err.Error(), http.StatusForbidden)
		return
	}
	type ReqPram struct {
		BankID string `json:"bank_id"`
		Price  int64  `json:"price"`
	}
	req := &ReqPram{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		Error(w, "can't parse body", http.StatusBadRequest)
		return
	}
	if req.Price == 0 {
		Error(w, "price is 0", http.StatusBadRequest)
		return
	}
	userID := s.filterBankID(w, req.BankID)
	if userID <= 0 {
		return
	}
	var rsvID int64
	price := req.Price
	memo := fmt.Sprintf("app:%s, price:%d", appid, req.Price)
	err = s.txScope(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`SELECT id FROM user WHERE id = ? LIMIT 1 FOR UPDATE`, userID); err != nil {
			return errors.Wrap(err, "select lock failed")
		}
		now := time.Now()
		expire := now.Add(5 * time.Minute)
		isMinus := price < 0
		if isMinus {
			var fixed, reserved int64
			if err := tx.QueryRow(`SELECT IFNULL(SUM(amount), 0) FROM credit WHERE user_id = ?`, userID).Scan(&fixed); err != nil {
				return errors.Wrap(err, "calc credit failed")
			}
			if err := tx.QueryRow(`SELECT IFNULL(SUM(amount), 0) FROM reserve WHERE user_id = ? AND is_minus = 1 AND expire_at >= ?`, userID, now).Scan(&reserved); err != nil {
				return errors.Wrap(err, "calc reserve failed")
			}
			if fixed+reserved+price < 0 {
				return CreditIsInsufficient
			}
		}
		query := `INSERT INTO reserve (user_id, amount, note, is_minus, created_at, expire_at) VALUES (?, ?, ?, ?, ?, ?)`
		sr, err := tx.Exec(query, userID, price, memo, isMinus, now, expire)
		if err != nil {
			return errors.Wrap(err, "update user.credit failed")
		}
		if rsvID, err = sr.LastInsertId(); err != nil {
			return errors.Wrap(err, "lastInsertID failed")
		}
		return nil
	})

	switch {
	case err == CreditIsInsufficient:
		Error(w, "credit is insufficient", http.StatusBadRequest)
	case err != nil:
		log.Printf("[WARN] reserve failed. err: %s", err)
		Error(w, "internal server error", http.StatusInternalServerError)
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprintln(w, fmt.Sprintf(`{"reserve_id":%d}`, rsvID))
	}
}

func (s *Handler) Commit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, err := appID(r)
	if err != nil {
		Error(w, err.Error(), http.StatusForbidden)
		return
	}
	type ReqPram struct {
		ReserveIDs []int64 `json:"reserve_ids"`
	}
	req := &ReqPram{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		Error(w, "can't parse body", http.StatusBadRequest)
		return
	}
	if len(req.ReserveIDs) == 0 {
		Error(w, "reserve_ids is required", http.StatusBadRequest)
		return
	}
	err = s.txScope(func(tx *sql.Tx) error {
		l := len(req.ReserveIDs)
		holder := "?" + strings.Repeat(",?", l-1)
		rids := make([]interface{}, l)
		for i, v := range req.ReserveIDs {
			rids[i] = v
		}
		// 空振りロックを避けるために個数チェック
		var count int
		query := fmt.Sprintf(`SELECT COUNT(id) FROM reserve WHERE id IN (%s) AND expire_at >= NOW()`, holder)
		if err := tx.QueryRow(query, rids...).Scan(&count); err != nil {
			return errors.Wrap(err, "count reserve failed")
		}
		if count < l {
			return ReserveIsExpires
		}

		// reserveの取得(for update)
		type Reserve struct {
			ID     int64
			UserID int64
			Amount int64
			Note   string
		}
		reserves := make([]Reserve, 0, l)
		query = fmt.Sprintf(`SELECT id, user_id, amount, note FROM reserve WHERE id IN (%s) FOR UPDATE`, holder)
		rows, err := tx.Query(query, rids...)
		if err != nil {
			return errors.Wrap(err, "select reserves failed")
		}
		defer rows.Close()
		for rows.Next() {
			reserve := Reserve{}
			if err := rows.Scan(&reserve.ID, &reserve.UserID, &reserve.Amount, &reserve.Note); err != nil {
				return errors.Wrap(err, "select reserves failed")
			}
			reserves = append(reserves, reserve)
		}
		if err = rows.Err(); err != nil {
			return errors.Wrap(err, "select reserves failed")
		}
		if len(reserves) != l {
			return ReserveIsAlreadyCommitted
		}

		// userのlock
		userids := make([]interface{}, l)
		for i, rsv := range reserves {
			userids[i] = rsv.UserID
		}
		query = fmt.Sprintf(`SELECT id FROM user WHERE id IN (%s)  LIMIT 1 FOR UPDATE`, holder)
		if _, err := tx.Exec(query, userids...); err != nil {
			return errors.Wrap(err, "select lock failed")
		}

		// 予約のcreditへの適用
		for _, rsv := range reserves {
			if err := s.modifyCredit(tx, rsv.UserID, rsv.Amount, rsv.Note); err != nil {
				return errors.Wrapf(err, "modifyCredit failed %#v", rsv)
			}
		}

		// reserveの削除
		query = fmt.Sprintf(`DELETE FROM reserve WHERE id IN (%s)`, holder)
		if _, err := tx.Exec(query, rids...); err != nil {
			return errors.Wrap(err, "delete reserve failed")
		}
		return nil
	})
	if err != nil {
		if err == ReserveIsExpires || err == ReserveIsAlreadyCommitted {
			Error(w, err.Error(), http.StatusBadRequest)
		} else {
			log.Printf("[WARN] commit credit failed. err: %s", err)
			Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}
	Success(w)
}

func (s *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, err := appID(r)
	if err != nil {
		Error(w, err.Error(), http.StatusForbidden)
		return
	}
	type ReqPram struct {
		ReserveIDs []int64 `json:"reserve_ids"`
	}
	req := &ReqPram{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		Error(w, "can't parse body", http.StatusBadRequest)
		return
	}
	if len(req.ReserveIDs) == 0 {
		Error(w, "reserve_ids is required", http.StatusBadRequest)
		return
	}
	err = s.txScope(func(tx *sql.Tx) error {
		l := len(req.ReserveIDs)
		holder := "?" + strings.Repeat(",?", l-1)
		rids := make([]interface{}, l)
		for i, v := range req.ReserveIDs {
			rids[i] = v
		}
		// 空振りロックを避けるために個数チェック
		var count int
		query := fmt.Sprintf(`SELECT COUNT(id) FROM reserve WHERE id IN (%s)`, holder)
		if err := tx.QueryRow(query, rids...).Scan(&count); err != nil {
			return errors.Wrap(err, "count reserve failed")
		}
		if count < l {
			return ReserveIsAlreadyCommitted
		}

		// reserveの取得(for update)
		type Reserve struct {
			ID     int64
			UserID int64
		}
		reserves := make([]Reserve, 0, l)
		query = fmt.Sprintf(`SELECT id, user_id FROM reserve WHERE id IN (%s) FOR UPDATE`, holder)
		rows, err := tx.Query(query, rids...)
		if err != nil {
			return errors.Wrap(err, "select reserves failed")
		}
		defer rows.Close()
		for rows.Next() {
			reserve := Reserve{}
			if err := rows.Scan(&reserve.ID, &reserve.UserID); err != nil {
				return errors.Wrap(err, "select reserves failed")
			}
			reserves = append(reserves, reserve)
		}
		if err = rows.Err(); err != nil {
			return errors.Wrap(err, "select reserves failed")
		}
		if len(reserves) != l {
			return ReserveIsAlreadyCommitted
		}

		// userのlock
		userids := make([]interface{}, l)
		for i, rsv := range reserves {
			userids[i] = rsv.UserID
		}
		query = fmt.Sprintf(`SELECT id FROM user WHERE id IN (%s)  LIMIT 1 FOR UPDATE`, holder)
		if _, err := tx.Exec(query, userids...); err != nil {
			return errors.Wrap(err, "select lock failed")
		}

		// reserveの削除
		query = fmt.Sprintf(`DELETE FROM reserve WHERE id IN (%s)`, holder)
		if _, err := tx.Exec(query, rids...); err != nil {
			return errors.Wrap(err, "delete reserve failed")
		}
		return nil
	})
	if err != nil {
		if err == ReserveIsExpires || err == ReserveIsAlreadyCommitted {
			Error(w, err.Error(), http.StatusBadRequest)
		} else {
			log.Printf("[WARN] cancel credit failed. err: %s", err)
			Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}
	Success(w)
}

func (s *Handler) filterBankID(w http.ResponseWriter, bankID string) int64 {
	if bankID == "" {
		Error(w, "bank_id is required", http.StatusBadRequest)
		return 0
	}
	cacheBankIDMutex.RLock()
	if id, ok := cacheBankID[bankID]; ok {
		cacheBankIDMutex.RUnlock()
		return id
	}
	cacheBankIDMutex.RUnlock()

	var id int64
	err := s.db.QueryRow(`SELECT id FROM user WHERE bank_id = ? LIMIT 1`, bankID).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		Error(w, "bank_id not found", http.StatusNotFound)
		return 0
	case err != nil:
		log.Printf("[WARN] get user failed. err: %s", err)
		Error(w, "internal server error", http.StatusInternalServerError)
		return 0 // クエリ失敗の時は cache しないで返る
	}
	cacheBankIDMutex.Lock()
	cacheBankID[bankID] = id
	cacheBankIDMutex.Unlock()
	return id
}

func (s *Handler) txScope(f func(*sql.Tx) error) (err error) {
	tx, err := s.db.Begin()
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

func (s *Handler) modifyCredit(tx *sql.Tx, userID, price int64, memo string) error {
	if _, err := tx.Exec(`INSERT INTO credit (user_id, amount, note, created_at) VALUES (?, ?, ?, NOW(6))`, userID, price, memo); err != nil {
		return errors.Wrap(err, "insert credit failed")
	}
	var credit int64
	if err := tx.QueryRow(`SELECT IFNULL(SUM(amount),0) FROM credit WHERE user_id = ?`, userID).Scan(&credit); err != nil {
		return errors.Wrap(err, "calc credit failed")
	}
	if _, err := tx.Exec(`UPDATE user SET credit = ? WHERE id = ?`, credit, userID); err != nil {
		return errors.Wrap(err, "update user.credit failed")
	}
	return nil
}

func (s *Handler) Initialize(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	queries := []string{
		`TRUNCATE user`,
		`TRUNCATE credit`,
		`TRUNCATE reserve`,
	}
	for _, query := range queries {
		log.Println("initialize", query)
		if _, err := s.db.Exec(query); err != nil {
			Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	Success(w)
}

func init() {
	var err error
	loc, err := time.LoadLocation(LocationName)
	if err != nil {
		log.Panicln(err)
	}
	time.Local = loc
}
