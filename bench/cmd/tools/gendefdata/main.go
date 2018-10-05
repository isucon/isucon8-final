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

	"github.com/ken39arg/isucon2018-final/bench"
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

func writeUserSQL(w io.Writer, users []User) error {
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
		dir = flag.String("dir", "isucondata", "output dir")
	)
	flag.Parse()
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Fatal(err)
	}
	time.Local = loc
	if err := run(*dir); err != nil {
		log.Fatal(err)
	}
}

func run(dir string) error {
	if err := os.MkdirAll(dir, 0775); err != nil {
		return err
	}

	banksql, err := os.Create(filepath.Join(dir, "bank.init.sql"))
	if err != nil {
		return err
	}
	defer banksql.Close()

	usersql, err := os.Create(filepath.Join(dir, "app.user.sql"))
	if err != nil {
		return err
	}
	defer usersql.Close()

	tradesql, err := os.Create(filepath.Join(dir, "app.trade.sql"))
	if err != nil {
		return err
	}
	defer tradesql.Close()

	ordersql, err := os.Create(filepath.Join(dir, "app.order.sql"))
	if err != nil {
		return err
	}
	defer ordersql.Close()

	r, err := bench.NewRandom()
	if err != nil {
		return err
	}

	type p struct {
		s, e string
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
		//tm              = time.Date(2019, 9, 20, 10, 0, 0, 0, time.Local)
		tm    = time.Date(2018, 10, 10, 10, 0, 0, 0, time.Local)
		end   = time.Date(2018, 10, 20, 10, 0, 0, 0, time.Local)
		eg    = new(errgroup.Group)
		pchan = make(chan p, 1000)
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i <= 10; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					pass := r.Password()
					ep, _ := bcrypt.GenerateFromPassword([]byte(pass), rand.Intn(3))
					pchan <- p{pass, string(ep)}
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
				return writeUserSQL(usersql, c)
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
		if len(users) < 100 || rand.Intn(50) == 1 {
			_p := <-pchan
			user := User{
				ID:        atomic.AddInt64(&userID, 1),
				Name:      r.Name(),
				BankID:    r.ID(),
				Password:  _p.s,
				pass:      _p.e,
				CreatedAt: tm,
			}
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
		if rand.Intn(100) == 1 {
			return keepusers[rand.Intn(len(keepusers))]
		}
		return users[rand.Intn(len(users))]
	}

	for tm.Before(end) {
		// random work
		switch rand.Intn(3) {
		case 0:
			price++
		case 1:
			price--
			if price < 3000 {
				price += 2
			}
		}
		tm = tm.Add(time.Millisecond * 50)
		u1 := pickUser(tm)

		tm = tm.Add(time.Millisecond * time.Duration(rand.Int63n(100)))
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
			order1.Price = trade.Price + 100 + rand.Int63n(100)
			order2.Price = trade.Price - 100 - rand.Int63n(100)
			order1.ClosedAt = tm.Add(time.Millisecond + time.Duration(rand.Int63n(20000)+800))
			order2.ClosedAt = tm.Add(time.Millisecond + time.Duration(rand.Int63n(20000)+800))
		}

		orders = append(orders, order1, order2)
		orders = writeOrder(orders, false)
		tm = tm.Add(time.Millisecond * time.Duration(rand.Int63n(500)+200))
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
