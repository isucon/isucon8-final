package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"isucon8/isucoin/controller"

	"github.com/gorilla/sessions"
	"github.com/julienschmidt/httprouter"
)

const (
	SessionSecret = "tonymoris"
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
		data   = getEnv("DATA_DIR", "data")
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
	server := newHandler(db, store, public, data)

	addr := ":" + port
	log.Printf("[INFO] start server %s", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}

func newHandler(db *sql.DB, store sessions.Store, publicdir, datadir string) http.Handler {

	h := &controller.Handler{
		db:      db,
		store:   store,
		datadir: datadir,
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
	router.NotFound = http.FileServer(http.Dir(publicdir)).ServeHTTP

	return h.CommonMiddleware(router)
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv("ISU_" + key); ok {
		return v
	}
	return def
}
