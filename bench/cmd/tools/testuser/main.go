package main

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	var (
		dsn = flag.String("dsn", "root:root@tcp(127.0.0.1:3306)/isucoin", "mysql dsn")
		tsv = flag.String("tsv", "isucondata/app.user.tsv", "user tsv")
		out = flag.String("code", "bench/testusers.go", "generated code")
	)
	flag.Parse()
	db, err := sql.Open("mysql", *dsn+"?parseTime=true&loc=Local&charset=utf8mb4")
	if err != nil {
		log.Fatal(err)
	}
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Panicln(err)
	}
	time.Local = loc
	r, err := os.Open(*tsv)
	if err != nil {
		log.Panicln(err)
	}
	w, err := os.Create(*out)
	if err != nil {
		log.Panicln(err)
	}
	if err = run(db, r, w); err != nil {
		log.Fatal(err)
	}
}

func run(db *sql.DB, r io.Reader, w io.Writer) error {
	type TestUser struct {
		BankID string
		Name   string
		Pass   string
		Cost   int
		Orders int
		Traded int
	}
	users := make([]TestUser, 0, 1000)

	fmt.Fprintln(w, "package bench")
	fmt.Fprintf(w, `
		type TestUser struct {
			BankID string
			Name   string
			Pass   string
			Cost   int
			Orders int
			Traded int
		}

		var testUsers = []TestUser{
	`)

	reader := csv.NewReader(r)
	reader.Comma = '\t'
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// id      name    bank    pass    bcript
		if record[0] == "id" {
			// header
			continue
		}
		var u TestUser
		u.Name = record[1]
		u.BankID = record[2]
		u.Pass = record[3]
		u.Cost, err = bcrypt.Cost([]byte(record[4]))
		if err != nil {
			return err
		}

		if len(users) > 300 && u.Cost < 8 {
			continue
		}
		if err = db.QueryRow("SELECT COUNT(*), COUNT(trade_id) FROM orders WHERE user_id = ?", record[0]).Scan(&u.Orders, &u.Traded); err != nil {
			return err
		}
		if u.Orders < 50 {
			// 注文が少ないのに興味は無い
			continue
		}
		d := fmt.Sprintf("%#v", u)
		fmt.Fprintf(w, "%s,\n", strings.TrimPrefix(d, "main."))
		users = append(users, u)
		if len(users) >= 1000 {
			break
		}
	}
	fmt.Fprintln(w, "}")

	return nil
}
