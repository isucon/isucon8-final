package main

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"flag"
	"io"
	"log"
	"math/rand"
	"os"
	"time"

	"bench"
)

var (
	appep        = flag.String("appep", "https://localhost.isucon8.flying-chair.net", "app endpoint")
	bankep       = flag.String("bankep", "https://compose.isucon8.flying-chair.net:5515", "isubank endpoint")
	logep        = flag.String("logep", "https://compose.isucon8.flying-chair.net:5516", "isulog endpoint")
	internalbank = flag.String("internalbank", "https://localhost.isucon8.flying-chair.net:5515", "isubank endpoint (for internal)")
	internallog  = flag.String("internallog", "https://localhost.isucon8.flying-chair.net:5516", "isulog endpoint (for internal)")
	jobid        = flag.String("jobid", "", "portal jobid")
	logoutput    = flag.String("log", "", "output log path (default stderr)")
	result       = flag.String("result", "", "result json path (default stdout)")
	teestdout    = flag.String("teestdout", "", "tee stdout")
	stateout     = flag.String("stateout", "", "save state filename")
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
	var (
		writer io.Writer
		tee    *os.File
	)
	if *teestdout != "" {
		tee, _ = os.Create(*teestdout)
		writer = io.MultiWriter(logout, tee)
		defer tee.Close()
	} else {
		writer = logout
	}
	mgr, err := bench.NewManager(writer, *appep, *bankep, *logep, *internalbank, *internallog, *stateout)
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
