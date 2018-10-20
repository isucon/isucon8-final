package bench

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"bench/isubank"
	"bench/isulog"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type PreTester struct {
	appep   string
	isulog  *isulog.Isulog
	isubank *isubank.Isubank
}

func (t *PreTester) Run(ctx context.Context) error {
	now := time.Now()
	eg := new(errgroup.Group)

	account1 := fmt.Sprintf("asuzuki%d@isucon.net", now.Unix())
	account2 := fmt.Sprintf("tmorris%d@isucon.net", now.Unix())
	name1, name2 := "鈴木 明", "トニー モリス"

	c1, err := NewClient(t.appep, account1, name1, "1234567890abc", ClientTimeout, RetireTimeout)
	if err != nil {
		return errors.Wrap(err, "create new client failed")
	}
	c2, err := NewClient(t.appep, account2, name2, "234567890abcd", ClientTimeout, RetireTimeout)
	if err != nil {
		return errors.Wrap(err, "create new client failed")
	}

	eg.Go(func() error {
		log.Printf("[INFO] run guest test")
		// Top
		if err := c2.Top(ctx); err != nil {
			return err
		}
		// 非ログイン /info
		info, err := c2.Info(ctx, 0)
		if err != nil {
			return err
		}

		if info.TradedOrders != nil && len(info.TradedOrders) > 0 {
			return errors.Errorf("GET /info ゲストユーザーのtraded_ordersが設定されています")
		}
		// 初期状態では0
		if info.LowestSellPrice < info.HighestBuyPrice {
			// 注文個数によってはあり得るのでそうならないシナリオにしたい
			return errors.Errorf("GET /info highest_buy_price と lowest_sell_price の関係が取引可能状態です")
		}
		// 初期データ件数は変動しない (TODO: 詳細もチェックするかどうか)
		log.Printf("[DEBUG] sec:%d, min:%d, hour:%d", len(info.ChartBySec), len(info.ChartByMin), len(info.ChartByHour))
		if len(info.ChartBySec) < 143 {
			return errors.Errorf("GET /info chart_by_sec の件数が初期データよりも少なくなっています")
		}
		if len(info.ChartByMin) < 300 {
			return errors.Errorf("GET /info chart_by_min の件数が初期データよりも少なくなっています")
		}
		if len(info.ChartByHour) < 48 {
			return errors.Errorf("GET /info chart_by_hour の件数が初期データよりも少なくなっています")
		}
		return nil
	})
	eg.Go(func() error {
		log.Printf("[INFO] run no acount test")
		err := c1.Signin(ctx)
		if err == nil {
			return errors.New("POST /signin 存在しないアカウントでログインに成功しました")
		}
		if e, ok := err.(*ErrorWithStatus); ok {
			if e.StatusCode != 404 {
				return errors.Errorf("POST /signin 失敗時のstatuscodeが正しくありません [%d]", e.StatusCode)
			}
		} else {
			return errors.Wrap(err, "POST /signin に失敗しました")
		}
		return nil
	})
	eg.Go(func() error {
		log.Printf("[INFO] run exists user test")
		gd := testUsers[rand.Intn(10)]
		gc, err := NewClient(t.appep, gd.BankID, gd.Name, gd.Pass, ClientTimeout, RetireTimeout)
		if err != nil {
			return errors.Wrap(err, "create new client failed")
		}
		if err := gc.Signin(ctx); err != nil {
			return errors.Wrapf(err, "Signin(bank:%s,name:%s)", gd.BankID, gd.Name)
		}
		info, err := gc.Info(ctx, 0)
		if err != nil {
			return err
		}
		if len(info.TradedOrders) < gd.Traded {
			return errors.Errorf("GET /info traded_ordersの件数が少ないです user:%d, got: %d, expected: %d", gc.UserID(), len(info.TradedOrders), gd.Traded)
		}
		orders, err := gc.GetOrders(ctx)
		if err != nil {
			return err
		}
		if o := len(orders); o < gd.Traded {
			return errors.Errorf("GET /orders 件数があいません user:%d, got: %d, expected: %d", gc.UserID(), o, gd.Traded)
		}
		count := 0
		for _, o := range orders {
			if o.Trade != nil {
				count++
			}
		}
		if count != len(info.TradedOrders) {
			return errors.Errorf("GET /orders trade が正しく設定されていない可能性があります")
		}
		return nil
	})

	eg.Go(func() error {
		log.Printf("[INFO] run bunk id not exist test")
		// BANK IDが存在しない
		err := c1.Signup(ctx)
		if err == nil {
			return errors.New("POST /signup 銀行に存在しないアカウントサインアップに成功しました。アカウントチェックを指定ない可能性があります")
		}
		if e, ok := err.(*ErrorWithStatus); ok {
			if e.StatusCode != 404 {
				return errors.Errorf("POST /signup statuscodeが正しくありません [%d]", e.StatusCode)
			}
		} else {
			return errors.Wrap(err, "POST /signup に失敗しました")
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	for _, id := range []string{account1, account2} {
		if err := t.isubank.NewBankID(id); err != nil {
			return errors.Wrap(err, "new bank_id failed")
		}
	}

	{
		log.Printf("[INFO] run signup and signin")
		eg := new(errgroup.Group)
		for _, c0 := range []*Client{c1, c2} {
			c := c0
			eg.Go(func() error {
				if err := c.Top(ctx); err != nil {
					return err
				}
				if _, err := c.Info(ctx, 0); err != nil {
					return err
				}
				if err := c.Signup(ctx); err != nil {
					return err
				}
				if err := c.Signin(ctx); err != nil {
					return err
				}
				if _, err := c.GetOrders(ctx); err != nil {
					return err
				}
				return nil
			})
		}
		if err := eg.Wait(); err != nil {
			return err
		}
	}

	{
		log.Printf("[INFO] run conflict test")
		c1x, err := NewClient(t.appep, account1, "鈴木 昭夫", "13467890abc", ClientTimeout, RetireTimeout)
		if err != nil {
			return errors.Wrap(err, "create new client failed")
		}
		err = c1x.Signup(ctx)
		if err == nil {
			return errors.New("POST /signup 重複アカウントでのサインアップに成功しました")
		}
		if e, ok := err.(*ErrorWithStatus); ok {
			if e.StatusCode != 409 {
				return errors.Errorf("POST /signup statuscodeが正しくありません [%d]", e.StatusCode)
			}
		} else {
			return errors.Wrap(err, "POST /signup に失敗しました")
		}
	}

	{
		log.Printf("[INFO] run buy order no money")
		order, err := c1.AddOrder(ctx, TradeTypeBuy, 1, 2000)
		if err == nil {
			return errors.Errorf("POST /orders 銀行に残高が足りない買い注文に成功しました [order_id:%d]", order.ID)
		}
		if e, ok := err.(*ErrorWithStatus); ok {
			if e.StatusCode != 400 {
				return errors.Errorf("POST /orders statuscodeが正しくありません [%d]", e.StatusCode)
			}
		} else {
			return errors.Wrap(err, "POST /orders に失敗しました")
		}
	}

	// 売り注文は成功する
	{
		log.Printf("[INFO] run sell order")
		o, err := c1.AddOrder(ctx, TradeTypeSell, 1, 1000)
		if err != nil {
			return err
		}
		orders, err := c1.GetOrders(ctx)
		if err != nil {
			return err
		}
		if g, w := len(orders), 1; g != w {
			return errors.Errorf("GET /orders 件数が正しくありません[got:%d, want:%d]", g, w)
		}
		if g, w := orders[0].ID, o.ID; g != w {
			return errors.Errorf("GET /orders IDが正しくありません[got:%d, want:%d]", g, w)
		}
		if g, w := orders[0].Price, o.Price; g != w {
			return errors.Errorf("GET /orders Priceが正しくありません[got:%d, want:%d]", g, w)
		}
		if g, w := orders[0].Amount, o.Amount; g != w {
			return errors.Errorf("GET /orders Amountが正しくありません[got:%d, want:%d]", g, w)
		}
		if g, w := orders[0].Type, o.Type; g != w {
			return errors.Errorf("GET /orders Typeが正しくありません[got:%s, want:%s]", g, w)
		}

		log.Printf("[INFO] run delete order")
		if err = c1.DeleteOrders(ctx, o.ID); err != nil {
			return err
		}
		orders, err = c1.GetOrders(ctx)
		if err != nil {
			return err
		}
		if g, w := len(orders), 0; g != w {
			return errors.Errorf("GET /orders 件数が正しくありません[got:%d, want:%d]", g, w)
		}
	}

	{
		log.Printf("[INFO] run trade matching")
		// 注文をして成立させる
		// 注文(敢えて並列にしない)
		if err := t.isubank.AddCredit(account1, 36000); err != nil {
			return err
		}
		for _, ap := range []struct {
			t     string
			c     *Client
			amont int64
			price int64
		}{
			{TradeTypeSell, c2, 2, 7001},
			{TradeTypeBuy, c1, 1, 6998},
			{TradeTypeSell, c2, 1, 6999},
			{TradeTypeBuy, c1, 2, 6999},
			{TradeTypeSell, c2, 1, 6999},
		} {
			var typeName string
			if ap.t == TradeTypeBuy {
				typeName = "買い注文"
			} else {
				typeName = "売り注文"
			}
			order, err := ap.c.AddOrder(ctx, ap.t, ap.amont, ap.price)
			if err != nil {
				return errors.Wrapf(err, "POST /orders %sに失敗しました [amount:%d, price:%d]", typeName, ap.amont, ap.price)
			}
			orders, err := ap.c.GetOrders(ctx)
			if err != nil {
				return err
			}
			if len(orders) == 0 || orders[len(orders)-1].ID != order.ID {
				return errors.Errorf("GET /orders %sが反映されていません got: %d, want: %d", typeName, orders[len(orders)-1].ID, order.ID)
			}
		}
		log.Printf("[INFO] end order")
		eg := new(errgroup.Group)
		eg.Go(func() error {
			log.Printf("[INFO] run c1 checker")
			err := func() error {
				timeout := time.After(TestTradeTimeout)
				for {
					select {
					case <-timeout:
						return errors.Errorf("成立すべき取引が成立しませんでした(c1) [user:%d]", c1.UserID())
					default:
						info, err := c1.Info(ctx, 0)
						if err != nil {
							return err
						}
						if len(info.TradedOrders) >= 1 {
							return nil
						}
						time.Sleep(PollingInterval)
					}
				}
			}()
			if err != nil {
				return err
			}
			log.Printf("[INFO] trade sucess OK(c1)")

			orders, err := c1.GetOrders(ctx)
			if err != nil {
				return err
			}
			if g, w := len(orders), 2; g != w {
				return errors.Errorf("GET /orders 件数があいません [got:%d, want:%d]", g, w)
			}
			if orders[1].Trade == nil {
				return errors.Errorf("GET /orders 成立した注文のtradeが設定されていません")
			}
			bought := orders[1].Trade.Price * 2
			time.Sleep(300 * time.Millisecond)
			rest, err := t.isubank.GetCredit(account1)
			if err != nil {
				return err
			}
			if rest+bought != 36000 {
				return errors.Errorf("銀行残高があいません [%d]", rest)
			}
			log.Printf("[INFO] 残高チェック OK(c1)")

			return func() error {
				timeout := time.After(LogAllowedDelay)
				for {
					select {
					case <-timeout:
						return errors.Errorf("ログが送信されていません(c1)")
					default:
						logs, err := t.isulog.GetUserLogs(c1.UserID())
						if err != nil {
							return errors.Wrap(err, "isulog get user logs failed")
						}
						ok, err := func() (bool, error) {
							var fl []*isulog.Log
							fl = filterLogs(logs, isulog.TagSignup)
							if len(fl) == 0 {
								return false, nil
							}
							if fl[0].Signup.Name != name1 {
								return false, errors.Errorf("log.signup のnameが正しくありません")
							}
							if fl[0].Signup.BankID != account1 {
								return false, errors.Errorf("log.signup のbank_idが正しくありません")
							}
							fl = filterLogs(logs, isulog.TagSignin)
							if len(fl) == 0 {
								return false, nil
							}
							fl = filterLogs(logs, isulog.TagBuyError)
							if len(fl) < 1 {
								return false, nil
							}
							if fl[0].BuyError.Amount != 1 || fl[0].BuyError.Price != 2000 {
								return false, errors.Errorf("log.buy.errorが正しくありません")
							}
							fl = filterLogs(logs, isulog.TagBuyOrder)
							if len(fl) < 2 {
								return false, nil
							}
							fl = filterLogs(logs, isulog.TagBuyTrade)
							if len(fl) < 1 {
								return false, nil
							}
							return true, nil
						}()
						if err != nil {
							return err
						}
						if ok {
							log.Printf("[INFO] ログチェック OK(c1)")
							return nil
						}
						time.Sleep(PollingInterval)
					}
				}
			}()
		})
		eg.Go(func() error {
			log.Printf("[INFO] run c2 checker")
			err := func() error {
				timeout := time.After(TestTradeTimeout)
				for {
					select {
					case <-timeout:
						return errors.Errorf("成立すべき取引が成立しませんでした(c2)")
					default:
						info, err := c2.Info(ctx, 0)
						if err != nil {
							return err
						}
						if len(info.TradedOrders) == 2 {
							return nil
						}
						time.Sleep(PollingInterval)
					}
				}
			}()
			if err != nil {
				return err
			}
			log.Printf("[INFO] trade sucess OK(c2)")

			orders, err := c2.GetOrders(ctx)
			if err != nil {
				return err
			}
			if g, w := len(orders), 3; g != w {
				return errors.Errorf("GET /orders 件数があいません [got:%d, want:%d]", g, w)
			}
			if orders[1].Trade == nil {
				return errors.Errorf("GET /orders 成立した注文のtradeが設定されていません")
			}
			if orders[2].Trade == nil {
				return errors.Errorf("GET /orders 成立した注文のtradeが設定されていません")
			}
			bought := orders[1].Trade.Price + orders[2].Trade.Price
			time.Sleep(300 * time.Millisecond)
			rest, err := t.isubank.GetCredit(account2)
			if err != nil {
				return err
			}
			if rest != bought {
				return errors.Errorf("銀行残高があいません [%d]", rest)
			}
			log.Printf("[INFO] 残高チェック OK(c2)")

			return func() error {
				timeout := time.After(LogAllowedDelay)
				var logs []*isulog.Log
				var err error
				for {
					select {
					case <-timeout:
						log.Printf("[DEBUG] logs % #v", logs)
						return errors.Errorf("ログが送信されていません(c2)")
					default:
						logs, err = t.isulog.GetUserLogs(c2.UserID())
						if err != nil {
							return errors.Wrap(err, "isulog get user logs failed")
						}
						ok, err := func() (bool, error) {
							var fl []*isulog.Log
							fl = filterLogs(logs, isulog.TagSignup)
							if len(fl) == 0 {
								return false, nil
							}
							if fl[0].Signup.Name != name2 {
								return false, errors.Errorf("log.signup のnameが正しくありません")
							}
							if fl[0].Signup.BankID != account2 {
								return false, errors.Errorf("log.signup のbank_idが正しくありません")
							}
							fl = filterLogs(logs, isulog.TagSignin)
							if len(fl) == 0 {
								return false, nil
							}
							fl = filterLogs(logs, isulog.TagSellOrder)
							if len(fl) < 3 {
								return false, nil
							}
							fl = filterLogs(logs, isulog.TagSellTrade)
							if len(fl) < 2 {
								return false, nil
							}
							return true, nil
						}()
						if err != nil {
							return err
						}
						if ok {
							log.Printf("[INFO] ログチェック OK(c2)")
							return nil
						}
						time.Sleep(PollingInterval)
					}
				}
			}()
		})
		if err := eg.Wait(); err != nil {
			return err
		}
		log.Printf("[INFO] 取引テストFinish")
	}

	return nil
}

type testUser interface {
	Orders() []*Order
	UserID() int64
	BankID() string
	Credit() int64
	FetchOrders(context.Context) error
	Ignore() bool
	Client() *Client
}

type PostTester struct {
	appep   string
	isulog  *isulog.Isulog
	isubank *isubank.Isubank
	users   []testUser
	tested  []testUser
}

func (t *PostTester) Run(ctx context.Context) error {
	users := make([]testUser, 0, len(t.users))
	for _, tu := range t.users {
		if tu.UserID() > 0 && !tu.Ignore() {
			users = append(users, tu)
		}
	}
	if len(users) == 0 {
		return errors.Errorf("ユーザーが全滅しています")
	}
	var trade *Trade
	{
		first, latest, random := users[0], users[len(users)-1], users[rand.Intn(len(users))]
		for len(users) >= 3 && (first.UserID() == random.UserID() || latest.UserID() == random.UserID()) {
			random = users[rand.Intn(len(users)-2)+1]
		}
		for _, user := range users {
			for _, order := range user.Orders() {
				if order.Trade != nil {
					if trade == nil || trade.CreatedAt.Before(order.Trade.CreatedAt) {
						trade = order.Trade
						latest = user
					}
				}
			}
		}
		if trade == nil {
			return errors.Errorf("取引に成功したユーザーが全滅しているか、一人もいません")
		}
		t.tested = []testUser{first, latest, random}
	}
	eg := new(errgroup.Group)
	for _, tu := range t.tested {
		user := tu
		eg.Go(func() error {
			if err := user.FetchOrders(ctx); err != nil {
				return errors.Wrapf(err, "注文情報の取得に失敗しました [user:%d]", user.UserID)
			}
			for _, order := range user.Orders() {
				if order.ClosedAt == nil {
					// 未成約の注文はキャンセルしておく
					o := order
					eg.Go(func() error {
						err := user.Client().DeleteOrders(ctx, o.ID)
						if er, ok := err.(*ErrorWithStatus); ok && er.StatusCode == 404 {
							err = nil
						}
						if o.ClosedAt == nil {
							now := time.Now()
							o.ClosedAt = &now
						}
						return err
					})
				}
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	deadline := time.Now().Add(LogAllowedDelay)

	eg = new(errgroup.Group)

	eg.Go(func() error {
		timeout := time.After(deadline.Sub(time.Now()))
		for {
			select {
			case <-timeout:
				return errors.Errorf("ログが欠損しています [trade:%d]", trade.ID)
			default:
				logs, err := t.isulog.GetTradeLogs(trade.ID)
				if err != nil {
					return errors.Wrap(err, "isulog get trade logs failed")
				}
				ok := func() bool {
					if countLog(logs, isulog.TagTrade) < 1 {
						return false
					}
					var bnum, snum int64
					for _, l := range filterLogs(logs, isulog.TagBuyTrade) {
						bnum += l.BuyTrade.Amount
					}
					for _, l := range filterLogs(logs, isulog.TagSellTrade) {
						snum += l.SellTrade.Amount
					}
					if bnum != trade.Amount || snum != trade.Amount {
						return false
					}
					return true
				}()
				if ok {
					log.Printf("[INFO] 取引ログチェックOK [trade:%d]", trade.ID)
					return nil
				}
			}
			time.Sleep(PollingInterval)
		}
	})
	for _, tu := range t.tested {

		user := tu
		eg.Go(func() error {
			timeout := time.After(deadline.Sub(time.Now()))
			var credit int64
			for credit != user.Credit() {
				select {
				case <-timeout:
					if credit == 0 {
						return errors.Errorf("処理がおそすぎてチェックの準備が整いませんでした[user:%d]", user.UserID())
					}
					log.Printf("[DEBUG] 銀行残高があいません [user:%d,bank:%s,bankCredit:%d,benchCredit:%d]", user.UserID(), user.BankID(), credit, user.Credit())
					return errors.Errorf("銀行残高があいません[user:%d]", user.UserID())
				default:
					var err error
					credit, err = t.isubank.GetCredit(user.BankID())
					if err != nil {
						return errors.Wrap(err, "ISUBANK APIとの通信に失敗しました")
					}
					if credit == user.Credit() {
						log.Printf("[INFO] 残高チェックOK (point1) [user:%d]", user.UserID())
						break
					}
					if err = user.FetchOrders(ctx); err != nil {
						return err
					}
					if credit == user.Credit() {
						log.Printf("[INFO] 残高チェックOK (point2) [user:%d]", user.UserID())
						break
					}
					time.Sleep(time.Millisecond * 500)
				}
			}
			var buy, sell, buyt, sellt, buyd, selld int
			for _, order := range user.Orders() {
				switch order.Type {
				case TradeTypeBuy:
					buy++
					if order.TradeID > 0 {
						buyt++
					} else if order.Removed() {
						buyd++
					}
				case TradeTypeSell:
					sell++
					if order.TradeID > 0 {
						sellt++
					} else if order.Removed() {
						selld++
					}
				}
			}
			for {
				select {
				case <-timeout:
					return errors.Errorf("ログが欠損しています [user:%d]", user.UserID())
				default:
					logs, err := t.isulog.GetUserLogs(user.UserID())
					if err != nil {
						return errors.Wrap(err, "isulog get user logs failed")
					}
					ok := func() bool {
						if c := countLog(logs, isulog.TagSignup); c == 0 {
							log.Printf("[INFO] not match log type: %s, nothing", isulog.TagSignup)
							return false
						}
						if c := countLog(logs, isulog.TagSignin); c == 0 {
							log.Printf("[INFO] not match log type: %s, nothing", isulog.TagSignin)
							return false
						}
						if c := countLog(logs, isulog.TagBuyOrder); c < buy {
							log.Printf("[INFO] not match log type: %s, %d < %d", isulog.TagBuyOrder, c, buy)
							return false
						}
						if c := countLog(logs, isulog.TagBuyTrade); c < buyt {
							log.Printf("[INFO] not match log type: %s, %d < %d", isulog.TagBuyTrade, c, buyt)
							return false
						}
						if c := countLog(logs, isulog.TagBuyDelete); c < buyd {
							log.Printf("[INFO] not match log type: %s, %d < %d", isulog.TagBuyDelete, c, buyd)
							return false
						}
						if c := countLog(logs, isulog.TagSellOrder); c < sell {
							log.Printf("[INFO] not match log type: %s, %d < %d", isulog.TagSellOrder, c, sell)
							return false
						}
						if c := countLog(logs, isulog.TagSellTrade); c < sellt {
							log.Printf("[INFO] not match log type: %s, %d < %d", isulog.TagSellTrade, c, sellt)
							return false
						}
						if c := countLog(logs, isulog.TagSellDelete); c < selld {
							log.Printf("[INFO] not match log type: %s, %d < %d", isulog.TagSellDelete, c, selld)
							return false
						}
						return true
					}()
					if ok {
						log.Printf("[INFO] ユーザーログチェックOK [user:%d]", user.UserID())
						return nil
					}
				}
				time.Sleep(PollingInterval)
			}
		})
	}

	return eg.Wait()
}

func filterLogs(logs []*isulog.Log, tag string) []*isulog.Log {
	ret := make([]*isulog.Log, 0, len(logs))
	for _, l := range logs {
		if l.Tag == tag {
			ret = append(ret, l)
		}
	}
	return ret
}

func countLog(logs []*isulog.Log, tag string) int {
	r := 0
	for _, l := range logs {
		if l.Tag == tag {
			r++
		}
	}
	return r
}
