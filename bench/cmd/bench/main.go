package main

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"flag"
	"math/rand"
	"os"
	"time"

	"github.com/kayac/inhouse-isucon-2018/bench"
)

var (
	host = flag.String("host", "localhost", "target host")
	port = flag.String("port", "5003", "target port")
	log  = bench.NewLogger(os.Stderr)
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	bm, err := bench.NewBenchmarker(os.Stderr, bench.BenchmarkerParams{
		Domain: "http://" + *host + ":" + *port,
		Time:   time.Minute,
	})
	if err != nil {
		return err
	}
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
