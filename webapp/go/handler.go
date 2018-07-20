package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

type Session struct {
	UserID int64
	BankID string
}

func NewServer(db *sql.DB, store sessions.Store) *http.ServeMux {
	server := http.NewServeMux()

	h := &Handler{
		db:    db,
		store: store,
	}

	server.HandleFunc("/initialize", h.Initialize)
	server.HandleFunc("/signup", h.Signup)
	server.HandleFunc("/signin", h.Signin)
	server.HandleFunc("/signout", h.authenticate(h.Signout))
	server.HandleFunc("/mypage", h.authenticate(h.MyPage))
	server.HandleFunc("/sell_requests", h.authenticate(h.SellRequests))
	server.HandleFunc("/buy_requests", h.authenticate(h.BuyRequests))
	server.HandleFunc("/trades", h.authenticate(h.Trades))

	// default 404
	server.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[INFO] request not found %s", r.URL.RawPath)
		http.Error(w, "Not found", 404)
	})
	return server
}

type Handler struct {
	db    *sql.DB
	store sessions.Store
}

func (h *Handler) Initialize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.handleError(w, err, http.StatusBadRequest)
		return
	}
	for _, k := range []string{
		BankEndpoint,
		BankAppid,
		LogEndpoint,
		LogAppid,
	} {
		if err := h.setSetting(k, r.FormValue(k)); err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
	}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			h.handleError(w, err, http.StatusBadRequest)
			return
		}
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
		pass, err := bcrypt.GenerateFromPassword([]byte(password), 31)
		if err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
		if res, err := h.db.Exec(`INSERT INTO user (bank_id, name, password, created_at) VALUES (?, ?, ? NOW())`, bankID, name, pass); err != nil {
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
		if err := r.ParseForm(); err != nil {
			h.handleError(w, err, http.StatusBadRequest)
			return
		}
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
	s := r.Context().Value("session").(*Session)
	_ = s
}

func (h *Handler) SellRequests(w http.ResponseWriter, r *http.Request) {
	s := r.Context().Value("session").(*Session)
	_ = s
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			h.handleError(w, err, http.StatusBadRequest)
			return
		}
	} else {
		// TODO list API
	}
}

func (h *Handler) BuyRequests(w http.ResponseWriter, r *http.Request) {
	s := r.Context().Value("session").(*Session)
	_ = s
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			h.handleError(w, err, http.StatusBadRequest)
			return
		}
	} else {
		// TODO list API
	}
}

func (h *Handler) Trades(w http.ResponseWriter, r *http.Request) {
	s := r.Context().Value("session").(*Session)
	_ = s
}

func (h *Handler) authenticate(f http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := h.store.Get(r, SessionName)
		if err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}
		if userID, ok := session.Values["user_id"]; ok {
			s := &Session{}
			s.UserID = userID.(int64)
			ctx := r.Context()
			if err := h.db.QueryRow(`SELECT bank_id FROM user WHERE id = ?`, userID).Scan(&s.BankID); err != nil {
				h.handleError(w, err, http.StatusInternalServerError)
				return
			}
			ctx = context.WithValue(ctx, "session", s)
			f.ServeHTTP(w, r.WithContext(ctx))
		} else {
			http.Redirect(w, r, "/signin", http.StatusFound)
		}
	})
}

func (h *Handler) handleError(w http.ResponseWriter, err error, code int) {
	log.Printf("[WARN] err: %s", err.Error())
	// TODO Error Message
	http.Error(w, err.Error(), code)
}

func (h *Handler) setSetting(k, v string) error {
	_, err := h.db.Exec(`INSERT INTO setting (key, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value)`, k, v)
	return err
}

func (h *Handler) getSetting(k string) (v string, err error) {
	err = h.db.QueryRow(`SELECT value FROM setting WHERE key = ?`, k).Scan(&v)
	return
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
