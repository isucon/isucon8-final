package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/ken39arg/isucon2018-final/bench/isubank"
)

func main() {
	bk, err := isubank.NewIsubank("https://localhost.isucon8.flying-chair.net:5515", "dummy")
	if err != nil {
		log.Fatal(err)
	}
	var ec, success int
	f := os.Stdin
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if 10 < ec {
			log.Fatal("too many errors")
		}
		line := scanner.Text()
		d := strings.Split(line, "\t")
		if d[0] == "id" || d[1] == "" {
			continue
		}
		if err := bk.NewBankID(d[1]); err != nil {
			log.Printf("bk.NewBankID failed %s", err)
			ec++
			continue
		}
		p, err := strconv.ParseInt(d[3], 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		if err := bk.AddCredit(d[1], p*10); err != nil {
			log.Printf("bk.AddCredit failed %s", err)
			ec++
			continue
		}
		success++
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	log.Printf("success: %d", success)
}
