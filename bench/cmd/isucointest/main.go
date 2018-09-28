package main

import (
	crand "crypto/rand"
	"encoding/binary"
	"flag"
	"math/rand"
	"os"
	"time"

	"github.com/ken39arg/isucon2018-final/bench"
	"github.com/pkg/errors"
)

var (
	appep        = flag.String("appep", "https://localhost.isucon8.flying-chair.net", "app endpoint")
	bankep       = flag.String("bankep", "https://compose.isucon8.flying-chair.net:5515", "isubank endpoint")
	logep        = flag.String("logep", "https://compose.isucon8.flying-chair.net:5516", "isulog endpoint")
	internalbank = flag.String("internalbank", "https://localhost.isucon8.flying-chair.net:5515", "isubank endpoint (for internal)")
	internallog  = flag.String("internallog", "https://localhost.isucon8.flying-chair.net:5516", "isulog endpoint (for internal)")
	log          = bench.NewLogger(os.Stderr)
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
	log.Printf("Success!")
}

func run() error {
	mgr, err := bench.NewManager(os.Stderr, *appep, *bankep, *logep, *internalbank, *internallog)
	if err != nil {
		return err
	}
	defer mgr.Close()
	log.Printf("run initialize")
	if err = mgr.Initialize(); err != nil {
		return errors.Wrap(err, "Initialize Failed")
	}

	log.Printf("run test")
	if err := mgr.PreTest(); err != nil {
		return errors.Wrap(err, "Test Failed")
	}
	return nil
}

func init() {
	var s int64
	if err := binary.Read(crand.Reader, binary.LittleEndian, &s); err != nil {
		s = time.Now().UnixNano()
	}
	rand.Seed(s)
}
