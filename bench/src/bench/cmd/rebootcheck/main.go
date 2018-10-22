package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"

	"bench"
)

var (
	stateout = flag.String("stateout", "", "save state filename")
)

func main() {
	flag.Parse()
	host, _ := os.Hostname()
	if err := run(); err != nil {
		log.Fatalf("%s is failed. err: %s", host, err)
	}
	log.Printf("%s is OK\n", host)
}

func run() error {
	ctx := context.Background()
	r, err := os.Open(*stateout)
	if err != nil {
		return err
	}
	defer r.Close()
	state := &bench.FinalState{}
	if err = json.NewDecoder(r).Decode(state); err != nil {
		return err
	}
	return state.Check(ctx)
}
