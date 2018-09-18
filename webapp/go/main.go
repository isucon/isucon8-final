package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/sessions"
)

func init() {
	var err error
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Panicln(err)
	}
	time.Local = loc
}

func main() {
	var (
		port   = getEnv("APP_PORT", "5000")
		dbhost = getEnv("DB_HOST", "127.0.0.1")
		dbport = getEnv("DB_PORT", "3306")
		dbuser = getEnv("DB_USER", "root")
		dbpass = getEnv("DB_PASSWORD", "")
		dbname = getEnv("DB_NAME", "isucoin")
		public = getEnv("PUBLIC_DIR", "public")
	)

	dbusrpass := dbuser
	if dbpass != "" {
		dbusrpass += ":" + dbpass
	}

	dsn := fmt.Sprintf(`%s@tcp(%s:%s)/%s?parseTime=true&loc=Local&charset=utf8mb4`, dbusrpass, dbhost, dbport, dbname)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("mysql connect failed. err: %s", err)
	}
	store := sessions.NewCookieStore([]byte(SessionSecret))
	server := NewServer(db, store, public)

	addr := ":" + port
	log.Printf("[INFO] start server %s", addr)
	//log.Fatal(http.ListenAndServe(addr, server))
	log.Fatal(http.ListenAndServe(addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		server.ServeHTTP(w, r)
		elasped := time.Now().Sub(start)
		log.Printf("%s\t%s\t%s\t%.5f", start.Format("2006-01-02T15:04:05.000"), r.Method, r.URL.Path, elasped.Seconds())
	})))
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv("ISU_" + key); ok {
		return v
	}
	return def
}
