package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	LocationName = "Asia/Tokyo"
)

var (
	port             = flag.Int("port", 14690, "log app running port")
	silent           = flag.Bool("silent", false, "disable request dump")
	logw   io.Writer = os.Stdout
)

func main() {
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	if *silent {
		logw = ioutil.Discard
	}
	server := http.NewServeMux()
	server.HandleFunc("/send", dumpHandler)
	server.HandleFunc("/send_bulk", dumpHandler)

	server.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[INFO] request not found %s", r.URL.RawPath)
		http.NotFound(w, r)
	})

	log.Printf("[INFO] start server %s", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}

func dumpHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(logw, "%s %s\n", r.Method, r.URL.Path)
	defer r.Body.Close()
	if _, err := io.Copy(logw, r.Body); err != nil {
		log.Printf("dump body failed")
	}
	fmt.Fprintf(logw, "--\n")
	fmt.Fprintln(w, "ok")
}

func init() {
	var err error
	loc, err := time.LoadLocation(LocationName)
	if err != nil {
		log.Panicln(err)
	}
	time.Local = loc
}
