package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/ken39arg/isucon2018-final/bench"
)

var (
	appep    = flag.String("appep", "https://localhost.isucon8.flying-chair.net", "app endpoint")
	stateout = flag.String("stateout", "", "save state filename")
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
	log.Printf("OK!")
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
	return state.Check(ctx, *appep)
}
