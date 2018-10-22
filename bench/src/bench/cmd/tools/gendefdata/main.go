package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"bench"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
)

type User struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	BankID    string `json:"-"`
	Password  string
	pass      string
	CreatedAt time.Time `json:"-"`
}

type Trade struct {
	ID        int64     `json:"id"`
	Amount    int64     `json:"amount"`
	Price     int64     `json:"price"`
	CreatedAt time.Time `json:"created_at"`
}

type Order struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	UserID    int64     `json:"user_id"`
	Amount    int64     `json:"amount"`
	Price     int64     `json:"price"`
	ClosedAt  time.Time `json:"closed_at"`
	TradeID   int64     `json:"trade_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Bank struct {
	ID        int64     `json:"id"`
	BankID    string    `json:"type"`
	Credit    int64     `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

const (
	DF = "2006-01-02 15:04:05.000000"
)

func writePartition(w io.Writer, table string, st, ed time.Time) error {
	if _, err := fmt.Fprintf(w, "ALTER TABLE %s DROP PRIMARY KEY, ADD PRIMARY KEY (id, created_at);", table); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "ALTER TABLE %s PARTITION BY RANGE COLUMNS(created_at) (", table); err != nil {
		return err
	}
	tm := st
	for tm.Before(ed) {
		if _, err := fmt.Fprintf(w, "PARTITION p%s VALUES LESS THAN ('%s'),", tm.Format("2006010215"), tm.Format("2006-01-02 15:00:00")); err != nil {
			return err
		}
		tm = tm.Add(time.Hour)
	}
	if _, err := fmt.Fprintln(w, "PARTITION pmax VALUES LESS THAN MAXVALUE);"); err != nil {
		return err
	}
	return nil
}

func writeBankSQL(w io.Writer, users []Bank) error {
	if _, err := fmt.Fprint(w, "INSERT INTO user (id,bank_id,credit,created_at) VALUES "); err != nil {
		return err
	}
	for i, user := range users {
		if i > 0 {
			if _, err := fmt.Fprint(w, ","); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "(%d,'%s',%d,'%s')", user.ID, user.BankID, user.Credit, user.CreatedAt.Format(DF)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, ";"); err != nil {
		return err
	}

	if _, err := fmt.Fprint(w, "INSERT INTO credit (user_id,amount,note,created_at) VALUES "); err != nil {
		return err
	}
	for i, user := range users {
		if i > 0 {
			if _, err := fmt.Fprint(w, ","); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "(%d,%d,'-','%s')", user.ID, user.Credit, user.CreatedAt.Format(DF)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, ";"); err != nil {
		return err
	}
	return nil
}

func writeUserSQL(w io.Writer, tsv io.Writer, users []User) error {
	if _, err := fmt.Fprint(w, "INSERT INTO user (id,bank_id,name,password,created_at) VALUES "); err != nil {
		return err
	}
	for i, user := range users {
		if i > 0 {
			if _, err := fmt.Fprint(w, ","); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "(%d,'%s','%s','%s','%s')", user.ID, user.BankID, user.Name, user.pass, user.CreatedAt.Format(DF)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tsv, "%d\t%s\t%s\t%s\t%s\n", user.ID, user.Name, user.BankID, user.Password, user.pass); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, ";")
	return err
}

func writeOrderSQL(w io.Writer, orders []Order) error {
	if _, err := fmt.Fprint(w, "INSERT INTO orders (id,type,user_id,amount,price,closed_at,trade_id,created_at) VALUES "); err != nil {
		return err
	}
	for i, order := range orders {
		if i > 0 {
			if _, err := fmt.Fprint(w, ","); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "(%d,'%s',%d,%d,%d", order.ID, order.Type, order.UserID, order.Amount, order.Price); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, ",'%s'", order.ClosedAt.Format(DF)); err != nil {
			return err
		}
		if order.TradeID == 0 {
			if _, err := fmt.Fprint(w, ",NULL"); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, ",%d", order.TradeID); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, ",'%s')", order.CreatedAt.Format(DF)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, ";")
	return err
}

func writeTradeSQL(w io.Writer, trades []Trade) error {
	if _, err := fmt.Fprint(w, "INSERT INTO trade (id,amount,price,created_at) VALUES "); err != nil {
		return err
	}
	for i, trade := range trades {
		if i > 0 {
			if _, err := fmt.Fprint(w, ","); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "(%d,%d,%d,'%s')", trade.ID, trade.Amount, trade.Price, trade.CreatedAt.Format(DF)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, ";")
	return err
}

func main() {
	var (
		dir   = flag.String("dir", "isucondata", "output dir")
		start = flag.String("start", "2018-10-11T10:00:00+09:00", "data start time RFC3339")
		end   = flag.String("end", "2018-10-16T10:00:00+09:00", "data end time RFC3339")
	)
	flag.Parse()
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Fatal(err)
	}
	time.Local = loc
	if err := run(*dir, *start, *end); err != nil {
		log.Fatal(err)
	}
}

func run(dir, starts, ends string) error {
	if err := os.MkdirAll(dir, 0775); err != nil {
		return err
	}
	banksql, err := os.Create(filepath.Join(dir, "bank.init.sql"))
	if err != nil {
		return err
	}
	defer banksql.Close()
	fmt.Fprintln(banksql, "use isubank;")
	fmt.Fprintln(banksql, "truncate credit;")
	fmt.Fprintln(banksql, "truncate reserve;")
	fmt.Fprintln(banksql, "truncate user;")

	usersql, err := os.Create(filepath.Join(dir, "app.user.sql"))
	if err != nil {
		return err
	}
	defer usersql.Close()
	fmt.Fprintln(usersql, "use isucoin;")
	fmt.Fprintln(usersql, "truncate user;")
	fmt.Fprintln(usersql, "set names utf8mb4;")
	usercsv, err := os.Create(filepath.Join(dir, "app.user.tsv"))
	if err != nil {
		return err
	}
	defer usercsv.Close()
	fmt.Fprintln(usercsv, "id\tname\tbank\tpass\tbcript")

	tradesql, err := os.Create(filepath.Join(dir, "app.trade.sql"))
	if err != nil {
		return err
	}
	defer tradesql.Close()
	fmt.Fprintln(tradesql, "use isucoin;")
	fmt.Fprintln(tradesql, "truncate trade;")

	ordersql, err := os.Create(filepath.Join(dir, "app.order.sql"))
	if err != nil {
		return err
	}
	defer ordersql.Close()
	fmt.Fprintln(ordersql, "use isucoin;")
	fmt.Fprintln(ordersql, "truncate orders;")

	tm, err := time.Parse(time.RFC3339, starts)
	if err != nil {
		return err
	}

	end, err := time.Parse(time.RFC3339, ends)
	if err != nil {
		return err
	}
	if err = writePartition(tradesql, "trade", tm, end.Add(time.Hour*38)); err != nil {
		return err
	}
	if err = writePartition(ordersql, "orders", tm, end.Add(time.Hour*38)); err != nil {
		return err
	}

	r, err := bench.NewRandom()
	if err != nil {
		return err
	}

	var (
		userID    int64 = 1234
		tradeID   int64 = 34123
		orderID   int64 = 123435
		bankID    int64 = 1
		tlock     sync.Mutex
		olock     sync.Mutex
		ulock     sync.Mutex
		block     sync.Mutex
		trades          = make([]Trade, 0, 10000)
		orders          = make([]Order, 0, 10000)
		users           = make([]User, 0, 10000)
		banks           = make([]Bank, 0, 10000)
		keepusers       = make([]User, 0, 10)
		price     int64 = 5000
		eg              = new(errgroup.Group)
		uchan           = make(chan User, 1000)
	)

	// ベンチの都合上固定
	uchan <- User{Name: "藍田 奈菜", BankID: "jz67jt77rpnb", Password: "7g39gnwr26ze", pass: "$2a$04$6ieL8BBW6oiDZAYOmdgViOR/026O9JHw7diR342/RyEhMhRI9IhFm"}
	uchan <- User{Name: "池野 歩", BankID: "2z82n5q", Password: "2s4s829vm2bg9", pass: "$2a$04$K4tqCfXVxQ7BUtC4Rx9S.Odc2LfjrJkv7ShMy5pYWQTqYNkIcKCgK"}
	uchan <- User{Name: "阿部 俊介", BankID: "k2vutw", Password: "kgt7e2yv863d5", pass: "$2a$04$qVEokzg7aANtQIn.R13Va.1vRDghvI7ChVA0J9cGsY0yq3hlxvZA6"}
	uchan <- User{Name: "古閑 麻美", BankID: "yft3f5d5g", Password: "5m99r6vt8qssunb7", pass: "$2a$04$24hSHJsvweeAx9CakOgume1YXxnBZTTGv2j0Z4mc41DJxH9wUM0za"}
	uchan <- User{Name: "川崎 大輝", BankID: "pcsuktmvqn", Password: "fkpcy2amcp9pkmx", pass: "$2a$04$MpuJEh8nrSyMpuxe7lp2ruGmBMtEjMwJcQJf3gFbVVzd/12z5kl.O"}
	uchan <- User{Name: "吉田 一", BankID: "hpnwwt", Password: "5y62vet3dcepg", pass: "$2a$04$KyozK1u71Gwh2Plpxkriwu6vizkuIxHJ2LIXpX4rlAZi/tnAVGuO."}
	uchan <- User{Name: "相田 大悟", BankID: "2q5m84je", Password: "qme4bak7x3ng", pass: "$2a$04$WnjM0.FfU47PvpwVwwnbiO3wS3vf6kieiNVDwZJHCWaLTZGtF4jem"}
	uchan <- User{Name: "泉 結子", BankID: "cymy39gqttm", Password: "8fnw4226kd63tv", pass: "$2a$04$SvScAwzL4kZQfYwu7.em6uGQg1hcxZMhk0aEFBZY97ILKAKOngE.K"}
	uchan <- User{Name: "谷本 楓花", BankID: "2e633gvuk8r", Password: "6f2fkzybgmhxynxp", pass: "$2a$04$D016sTFwcpsrLsV8DN5lqu.SRl/.YIHLzscaw4mMl6nxO0blETFK6"}
	uchan <- User{Name: "桑原 楓花", BankID: "qdyj7z5vj5", Password: "54f67y4exumtw", pass: "$2a$05$4nuH6tXyHkzNagtFyBJK9ubAhUKqE32EnOUo5PkYVTWtqdXP8aT8."}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i <= 50; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					cost := rand.Intn(11)
					if cost < 4 {
						cost = 4
					}
					pass := r.Password()
					ep, _ := bcrypt.GenerateFromPassword([]byte(pass), cost)
					uchan <- User{
						Name:     r.Name(),
						BankID:   r.ID(),
						Password: pass,
						pass:     string(ep),
					}
				}
			}
		}()
	}

	for i := 1; i <= 100; i++ {
		banks = append(banks, Bank{
			ID:        atomic.AddInt64(&bankID, 1),
			BankID:    fmt.Sprintf("isucon-%03d", i),
			Credit:    100000000,
			CreatedAt: tm,
		})
	}
	writeTrade := func(d []Trade, force bool) []Trade {
		if force || len(d) == cap(d) {
			c := make([]Trade, len(d))
			copy(c, d)
			eg.Go(func() error {
				tlock.Lock()
				defer tlock.Unlock()
				log.Printf("write trade %s %s", c[0].CreatedAt.Format(DF), c[len(c)-1].CreatedAt.Format(DF))
				return writeTradeSQL(tradesql, c)
			})
			return d[:0]
		}
		return d
	}
	writeOrder := func(d []Order, force bool) []Order {
		if len(d) > 0 && (force || len(d) == cap(d)) {
			c := make([]Order, len(d))
			copy(c, d)
			eg.Go(func() error {
				olock.Lock()
				defer olock.Unlock()
				log.Printf("write order %s %s", c[0].CreatedAt.Format(DF), c[len(c)-1].CreatedAt.Format(DF))
				return writeOrderSQL(ordersql, c)
			})
			return d[:0]
		}
		return d
	}
	writeUser := func(d []User, force bool) []User {
		if force || len(d) == cap(d) {
			c := make([]User, len(d))
			copy(c, d)
			eg.Go(func() error {
				ulock.Lock()
				defer ulock.Unlock()
				log.Printf("write user %s %s", c[0].CreatedAt.Format(DF), c[len(c)-1].CreatedAt.Format(DF))
				return writeUserSQL(usersql, usercsv, c)
			})
			return d[:0]
		}
		return d
	}
	writeBank := func(d []Bank, force bool) []Bank {
		if force || len(d) == cap(d) {
			c := make([]Bank, len(d))
			copy(c, d)
			eg.Go(func() error {
				block.Lock()
				defer block.Unlock()
				log.Printf("write bank %s %s", c[0].CreatedAt.Format(DF), c[len(c)-1].CreatedAt.Format(DF))
				return writeBankSQL(banksql, c)
			})
			return d[:0]
		}
		return d
	}

	pickUser := func(tm time.Time) User {
		if len(users) < 50 || rand.Intn(50) == 1 {
			user := <-uchan
			user.ID = atomic.AddInt64(&userID, 1)
			user.CreatedAt = tm
			users = append(users, user)
			banks = append(banks, Bank{
				ID:        atomic.AddInt64(&bankID, 1),
				BankID:    user.BankID,
				Credit:    100000000,
				CreatedAt: tm,
			})
			if len(keepusers) < cap(keepusers) {
				keepusers = append(keepusers, user)
			}
			users = writeUser(users, false)
			banks = writeBank(banks, false)
			return user
		}
		if rand.Intn(200) == 1 {
			return keepusers[rand.Intn(len(keepusers))]
		}
		return users[rand.Intn(len(users))]
	}

	for tm.Before(end) {
		// random work
		switch rand.Intn(10) {
		case 0, 1, 2, 3:
			price++
		case 5, 6, 7:
			price--
		default:
			if price > 7000 {
				price--
			} else if price < 4000 {
				price++
			}
		}
		tm = tm.Add(time.Millisecond * 50)
		u1 := pickUser(tm)

		tm = tm.Add(time.Millisecond * time.Duration(rand.Int63n(100)+50))
		u2 := pickUser(tm)

		tm = tm.Add(time.Duration(rand.Int63n(500)+200) * time.Millisecond)
		var trade Trade
		if rand.Intn(5) > 0 {
			// 成立
			trade = Trade{
				ID:        atomic.AddInt64(&tradeID, 1),
				Amount:    1,
				Price:     price,
				CreatedAt: tm,
			}
			trades = append(trades, trade)
			trades = writeTrade(trades, false)
		}
		order1 := Order{
			ID:        atomic.AddInt64(&orderID, 1),
			Type:      bench.TradeTypeSell,
			UserID:    u1.ID,
			Amount:    1,
			CreatedAt: tm.Add(time.Millisecond * -123),
		}
		order2 := Order{
			ID:        atomic.AddInt64(&orderID, 1),
			Type:      bench.TradeTypeBuy,
			UserID:    u2.ID,
			Amount:    1,
			CreatedAt: tm.Add(time.Millisecond * -56),
		}

		if trade.ID > 0 {
			order1.TradeID = trade.ID
			order2.TradeID = trade.ID
			order1.Price = trade.Price
			order2.Price = trade.Price
			order1.ClosedAt = trade.CreatedAt
			order2.ClosedAt = trade.CreatedAt
		} else {
			order1.Price = price + 100 + rand.Int63n(100)
			order2.Price = price - 100 - rand.Int63n(100)
			order1.ClosedAt = tm.Add(time.Millisecond + time.Duration(rand.Int63n(2000)+800))
			order2.ClosedAt = tm.Add(time.Millisecond + time.Duration(rand.Int63n(2000)+800))
		}

		orders = append(orders, order1, order2)
		orders = writeOrder(orders, false)
		switch rand.Intn(10) {
		case 1, 2, 3:
			tm = tm.Add(time.Millisecond * time.Duration(rand.Int63n(300)+500))
		case 8:
			tm = tm.Add(time.Millisecond * time.Duration(rand.Int63n(1000)+1500))
		case 9:
			tm = tm.Add(time.Millisecond * time.Duration(rand.Int63n(500)+1000))
		}
		tm = tm.Add(time.Millisecond * time.Duration(rand.Int63n(300)+200))
	}

	writeUser(users, true)
	writeTrade(trades, true)
	writeOrder(orders, true)
	writeBank(banks, true)
	log.Printf("Complete loop !")
	if err := eg.Wait(); err != nil {
		return err
	}
	log.Printf("Complete !")
	return nil
}
