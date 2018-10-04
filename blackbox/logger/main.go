package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

const (
	MaxBodySize  = 1024 * 1024 // 1MB
	LocationName = "Asia/Tokyo"
	AxLog        = false

	AppIDCtxKey            = "appid"
	initialStorageCapacity = 100000
	sendDelay              = 100 * time.Millisecond
)

var logStorage = NewStorage()
var mu sync.Mutex

func main() {
	var (
		port = flag.Int("port", 5516, "log app running port")
	)

	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	server := NewServer()

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

func NewServer() http.Handler {
	server := http.NewServeMux()

	h := &Handler{
		guard:   make(map[string]chan struct{}, 1000),
		waiting: make(map[string]*int64, 1000),
	}

	server.HandleFunc("/send", h.Send)
	server.HandleFunc("/send_bulk", h.SendBulk)
	server.HandleFunc("/logs", h.Logs)
	server.HandleFunc("/initialize", h.Initialize)

	// default 404
	server.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[INFO] request not found %s", r.URL.RawPath)
		Error(w, "Not found", 404)
	})
	s := authHandler(server)
	return http.HandlerFunc(s.ServeHTTP)
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
	log.Println(code, err)
	http.Error(w, err, code)
}

func Success(w http.ResponseWriter) {
	fmt.Fprintln(w, "ok")
}

type Log struct {
	Tag  string                 `json:"tag"`
	Time time.Time              `json:"time"`
	Data map[string]interface{} `json:"data"`
}

func (l Log) validate() error {
	if l.Tag == "" {
		return errors.New("empty tag")
	}
	if len(l.Data) == 0 {
		return errors.New("empty data")
	}
	return nil
}

type Handler struct {
	guard   map[string]chan struct{}
	waiting map[string]*int64
	mux     sync.Mutex
}

type Storage struct {
	mu   sync.Mutex
	logs map[string][]Log
}

func NewStorage() *Storage {
	return &Storage{
		logs: make(map[string][]Log),
	}
}

func (s *Storage) Append(appid string, l Log) {
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, ok := s.logs[appid]
	if !ok {
		s.logs[appid] = []Log{l}
	} else {
		s.logs[appid] = append(logs, l)
	}
}

func (s *Storage) AppendBulk(appid string, ls []Log) {
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, ok := s.logs[appid]
	if !ok {
		s.logs[appid] = ls
	} else {
		s.logs[appid] = append(logs, ls...)
	}
}

func (s *Storage) Search(appid string, userid, tradeid int64) []Log {
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, ok := s.logs[appid]
	if !ok {
		return []Log{}
	}
	ret := make([]Log, 0, len(logs))
LOGS:
	for _, l := range logs {
		if userid != 0 {
			if v, ok := l.Data["user_id"].(float64); ok {
				if float64(userid) != v {
					continue LOGS
				}
			}
		}
		if tradeid != 0 {
			if v, ok := l.Data["trade_id"].(float64); ok {
				if float64(tradeid) != v {
					continue LOGS
				}
			}
		}
		ret = append(ret, l)
	}
	return ret
}

func (s *Handler) Send(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, "", http.StatusMethodNotAllowed)
		return
	}
	appid, err := appID(r)
	if err != nil {
		Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	l := Log{}
	if err = json.NewDecoder(r.Body).Decode(&l); err != nil {
		Error(w, fmt.Sprintf("can't parse body. err:%s", err.Error()), http.StatusBadRequest)
		return
	}
	if err := l.validate(); err != nil {
		Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	logStorage.Append(appid, l)
	time.Sleep(sendDelay)
	Success(w)
}

func (s *Handler) SendBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, "", http.StatusMethodNotAllowed)
		return
	}
	appid, err := appID(r)
	if err != nil {
		Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	logs := []Log{}
	size, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if size > MaxBodySize {
		Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodySize)).Decode(&logs); err != nil {
		Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, l := range logs {
		if err := l.validate(); err != nil {
			Error(w, fmt.Sprintf("invalid log. err:%s", err), http.StatusBadRequest)
			return
		}
	}
	logStorage.AppendBulk(appid, logs)
	time.Sleep(sendDelay)
	Success(w)
}

func (s *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	args := make([]interface{}, 0, 3)
	var userid, tradeid int64

	appid := r.URL.Query().Get("app_id")
	if appid == "" {
		Error(w, "app_id required", http.StatusBadRequest)
		return
	}

	if _userid := r.URL.Query().Get("user_id"); _userid != "" {
		var err error
		userid, err = strconv.ParseInt(_userid, 10, 64)
		if err != nil {
			Error(w, "parse user_id failed", http.StatusBadRequest)
			return
		}
		args = append(args, userid)
	}
	if _tradeid := r.URL.Query().Get("trade_id"); _tradeid != "" {
		var err error
		tradeid, err = strconv.ParseInt(_tradeid, 10, 64)
		if err != nil {
			Error(w, "parse trade_id failed", http.StatusBadRequest)
			return
		}
		args = append(args, tradeid)
	}
	logs := logStorage.Search(appid, userid, tradeid)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(logs)
}

func (s *Handler) Initialize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, "", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	logStorage = NewStorage()
	mu.Unlock()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintln(w, `{"ok":true}`)
}

func init() {
	var err error
	loc, err := time.LoadLocation(LocationName)
	if err != nil {
		log.Panicln(err)
	}
	time.Local = loc
}
