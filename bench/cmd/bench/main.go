package main

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/ken39arg/isucon2018-final/bench"
)

var (
	appep        = flag.String("appep", "https://localhost.isucon8.flying-chair.net", "app endpoint")
	bankep       = flag.String("bankep", "https://compose.isucon8.flying-chair.net:5515", "isubank endpoint")
	logep        = flag.String("logep", "https://compose.isucon8.flying-chair.net:5516", "isulog endpoint")
	internalbank = flag.String("internalbank", "https://localhost.isucon8.flying-chair.net:5515", "isubank endpoint (for internal)")
	internallog  = flag.String("internallog", "https://localhost.isucon8.flying-chair.net:5516", "isulog endpoint (for internal)")
	jobid        = flag.String("jobid", "", "portal jobid")
	logoutput    = flag.String("log", "", "output log path (default stderror)")
	result       = flag.String("result", "", "result json path (default stdout)")
	logout       = os.Stderr
	out          = os.Stdout
)

func main() {
	flag.Parse()
	var err error
	if *result != "" {
		out, err = os.Create(*result)
		if err != nil {
			log.Fatal(err)
		}
		defer out.Close()
	}
	if *logoutput != "" {
		logout, err = os.Create(*logoutput)
		if err != nil {
			log.Fatal(err)
		}
		defer logout.Close()
	}
	log.SetOutput(logout)
	if err = run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	mgr, err := bench.NewManager(logout, *appep, *bankep, *logep, *internalbank, *internallog)
	if err != nil {
		return err
	}
	defer mgr.Close()
	msg := "ok"
	bm := bench.NewRunner(mgr)
	if err = bm.Run(context.Background()); err != nil {
		msg = err.Error()
		mgr.Logger().Printf(msg)
	}
	result := bm.Result()
	result.JobID = *jobid
	result.IPAddrs = *appep
	result.Message = msg
	json.NewEncoder(out).Encode(result)
	return nil
}

func init() {
	var s int64
	if err := binary.Read(crand.Reader, binary.LittleEndian, &s); err != nil {
		s = time.Now().UnixNano()
	}
	rand.Seed(s)
}
