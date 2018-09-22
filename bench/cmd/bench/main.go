package main

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"flag"
	"math/rand"
	"os"
	"time"

	"github.com/ken39arg/isucon2018-final/bench"
)

var (
	appep        = flag.String("appep", "https://localhost.isucon8.flying-chair.net", "app endpoint")
	bankep       = flag.String("bankep", "https://compose.isucon8.flying-chair.net:5515", "isubank endpoint")
	logep        = flag.String("logep", "https://compose.isucon8.flying-chair.net:5516", "isulog endpoint")
	internalbank = flag.String("internalbank", "https://localhost.isucon8.flying-chair.net:5515", "isubank endpoint")
	log          = bench.NewLogger(os.Stderr)
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	mgr, err := bench.NewManager(os.Stderr, *appep, *bankep, *logep, *internalbank)
	if err != nil {
		return err
	}
	bm := bench.NewRunner(mgr, time.Minute, 20*time.Millisecond)
	if err = bm.Run(context.Background()); err != nil {
		return err
	}
	bm.Result()
	return nil
}

func init() {
	var s int64
	if err := binary.Read(crand.Reader, binary.LittleEndian, &s); err != nil {
		s = time.Now().UnixNano()
	}
	rand.Seed(s)
}
