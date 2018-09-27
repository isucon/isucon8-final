package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/ken39arg/isucon2018-final/bench"
)

const (
	TO = 10 * time.Second
)

var (
	appep  = flag.String("appep", "https://localhost.isucon8.flying-chair.net", "app endpoint")
	bankep = flag.String("bankep", "http://mockservice:14809", "isubank endpoint")
	logep  = flag.String("logep", "http://mockservice:14690", "isulog endpoint")
)

type account struct {
	ID       string
	Password string
	Name     string
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	r, err := bench.NewRandom()
	if err != nil {
		return err
	}

	{
		// init
		guest, err := bench.NewClient(*appep, "", "", "", TO, TO)
		if err != nil {
			return err
		}
		if err := guest.Initialize(*bankep, r.ID(), *logep, r.ID()); err != nil {
			return err
		}
	}

	// ユーザーを沢山作る
	users := make([]account, 0, 1234)
	err = func() error {
		file, err := os.Create(`/tmp/userlist.csv`)
		if err != nil {
			return err
		}
		defer file.Close()
		for i := 0; i < 1234; i++ {
			a := account{
				ID:       r.ID(),
				Password: r.Password(),
				Name:     r.Name(),
			}
			c, err := bench.NewClient(*appep, a.ID, a.Name, a.Password, TO, TO)
			if err != nil {
				return err
			}
			if err = c.Signup(); err != nil {
				return err
			}
			users = append(users, a)
			if _, err := fmt.Fprintf(file, "%s,%s,%s\n", a.ID, a.Name, a.Password); err != nil {
				return err
			}
		}
		return nil
	}()
	if err != nil {
		return err
	}

	// ランダムウォークで取引を行う
	var price int64 = 5000
	for i := 0; i < 1000; i++ {
		// setup
		clients := make([]*bench.Client, 10)
		for i := range clients {
			a := users[rand.Intn(len(users))]
			if clients[i], err = bench.NewClient(*appep, a.ID, a.Name, a.Password, TO, TO); err != nil {
				return err
			}
			if err := clients[i].Signin(); err != nil {
				return err
			}
		}
		for j := 0; j < 100; j++ {
			if rand.Intn(2) == 1 {
				price++
			} else {
				price--
			}
			c := clients[rand.Intn(len(clients))]
			if j%2 == 1 {
				if _, err := c.AddOrder(bench.TradeTypeBuy, 1, price); err != nil {
					log.Printf("[WARN] add BuyOrder failed. err:%s", err)
				}
			} else {
				if _, err := c.AddOrder(bench.TradeTypeSell, 1, price); err != nil {
					log.Printf("[WARN] add SellOrder failed. err:%s", err)
				}
			}
			time.Sleep(time.Millisecond * 10)
		}
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}
