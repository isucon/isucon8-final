package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

func main() {
	var (
		port   = flag.Int("port", 5515, "bank app ranning port")
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

	log.Fatal(http.ListenAndServe(addr, server))
}

func NewServer(db *sql.DB) *http.ServeMux {
	server := http.NewServeMux()

	h := &Handler{db}

	h.HandleFunc("/register", c.Register)
	h.HandleFunc("/add_credit", c.AddCredit)
	h.HandleFunc("/check_credit", c.CheckCredit)
	h.HandleFunc("/send_credit", c.SendCredit)
	h.HandleFunc("/pull_credit", c.PullCredit)

	// default 404
	server.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[INFO] request not found %s", r.URL.RawPath)
		Error(w, "Not found", 404)
	})
	return server
}

const (
	ResOK    = `{"status":"ok"}`
	ResError = `{"status":"ng","error":"%s"}`
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

type ReqPram struct {
	AppID  string `json:"app_id"`
	BankID string `json:"bank_id"`
	Price  int64  `json:"price"`
}

type Handler struct {
	db *sql.DB
}

func (s *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req := &ReqPram{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		Error(w, "can't parse body", http.StatusBadRequest)
		return
	}
	if req.BankID == "" {
		Error(w, "bank_id is required", http.StatusBadRequest)
		return
	}
	if _, err := s.db.Exec(`INSERT INTO user (bank_id, created_at) VALUES (?, NOW())`, req.BankID); err != nil {
		if mysqlError, ok := err.(*mysql.MySQLError); ok {
			if mysqlError.Number == 1062 {
				Error(w, "bank_id already exists", http.StatusBadRequest)
				return
			}
		}
		log.Printf("[WARN] insert user failed. err: %s", err)
		Error(w, "internal server error", http.StatusInternalServerError)
	}
	Success(w)
}

func (s *Handler) AddCredit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req := &ReqPram{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		Error(w, "can't parse body", http.StatusBadRequest)
		return
	}
	if req.BankID == "" {
		Error(w, "bank_id is required", http.StatusBadRequest)
		return
	}
	if req.Price <= 0 {
		Error(w, "price must be upper than 0", http.StatusBadRequest)
		return
	}
	userID, _, err := s.getUserID(req.BankID)
	switch {
	case err == sql.ErrNoRows:
		Error(w, "user not found", http.StatusNotFound)
		return
	case err != nil:
		log.Printf("[WARN] get user failed. err: %s", err)
		Error(w, "internal server error", http.StatusInternalServerError)
	}

	if err = s.modifyPrice(userID, req.Price, "by add credit API"); err != nil {
		log.Printf("[WARN] modifyPrice failed. err: %s", err)
		Error(w, "internal server error", http.StatusInternalServerError)
	}
	Success(w)
}

func (s *Handler) CheckCredit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req := &ReqPram{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		Error(w, "can't parse body", http.StatusBadRequest)
		return
	}
	if req.BankID == "" {
		Error(w, "bank_id is required", http.StatusBadRequest)
		return
	}
	if req.Price <= 0 {
		Error(w, "price must be upper than 0", http.StatusBadRequest)
		return
	}
	userID, credit, err := s.getUserID(req.BankID)
	switch {
	case err == sql.ErrNoRows:
		Error(w, "user not found", http.StatusNotFound)
		return
	case err != nil:
		log.Printf("[WARN] get user failed. err: %s", err)
		Error(w, "internal server error", http.StatusInternalServerError)
	}
	if credit >= req.Price {
		Success(w)
	} else {
		Error(w, "credit is shorten", http.StatusOK)
	}
}

func (s *Handler) SendCredit(w http.ResponseWriter, r *http.Request) {
	Success(w)
}

func (s *Handler) PullCredit(w http.ResponseWriter, r *http.Request) {
	Success(w)
}

func (s *Handler) getUserID(bankID string) (id, credit int64, err error) {
	err = s.db.QueryRow(`SELECT id, credit FROM user WHERE bank_id = ? LIMIT 1`, bankID).Scan(&id, &credit)
	return
}

func (s *Handler) modifyPrice(userID, price int64, memo string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return errors.Wrap(err, "begin transaction failed")
	}
	if _, err = tx.Exec(`INSERT INTO jpyen (user_id, amount, note, created_at) VALUES (?, NOW())`, userID, price, memo); err != nil {
		return errors.Wrap(err, "insert jpyen failed")
	}
	var price int64
	if err = tx.QueryRow(`SELECT SUM(amount) FROM jpyen WHERE user_id = ?`, userID).Scan(&price); err != nil {
		return errors.Wrap(err, "calc jpyen failed")
	}
	if _, err = tx.Exec(`UPDATE user SET credit = ? WHERE user_id = ?`, price, bankID); err != nil {
		return errors.Wrap(err, "update user.credit failed")
	}
	return tx.Commit()
}
