package bench

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/ken39arg/isucon2018-final/bench/isubank"
	"github.com/ken39arg/isucon2018-final/bench/isulog"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type PreTester struct {
	appep   string
	isulog  *isulog.Isulog
	isubank *isubank.Isubank
}

func (t *PreTester) Run(ctx context.Context) error {
	// TODO: 並列化できるところをする
	now := time.Now()

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

	// Top
	if err := c2.Top(ctx); err != nil {
		return err
	}

	{
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
		// 10時以降のデータは消えるので件数は変動する(特にsecもminも消える)
		// if len(info.ChartBySec) < 5742 {
		// 	return errors.Errorf("GET /info chart_by_sec の件数が初期データよりも少なくなっています")
		// }
		// if len(info.ChartByMin) < 98 {
		// 	return errors.Errorf("GET /info chart_by_min の件数が初期データよりも少なくなっています")
		// }
		// if len(info.ChartByHour) < 2 {
		// 	return errors.Errorf("GET /info chart_by_hour の件数が初期データよりも少なくなっています")
		// }
	}
	{
		// アカウントがない
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
	}
	{
		// 既存ユーザー
		// | id   | name      | bank_id      | o   | t   | pass
		// | 1235 | 藍田 奈菜 | jz67jt77rpnb | 501 | 424 | 7g39gnwr26ze
		// | 1236 | 池野 歩   | 2z82n5q      | 559 | 459 | 2s4s829vm2bg9
		// | 1237 | 阿部 俊介 | k2vutw       | 557 | 449 | kgt7e2yv863d5
		// | 1238 | 古閑 麻美 | yft3f5d5g    | 543 | 422 | 5m99r6vt8qssunb7
		// | 1239 | 川崎 大輝 | pcsuktmvqn   | 549 | 443 | fkpcy2amcp9pkmx
		// | 1240 | 吉田 一   | hpnwwt       | 547 | 447 | 5y62vet3dcepg
		// | 1241 | 相田 大悟 | 2q5m84je     | 521 | 420 | qme4bak7x3ng
		// | 1242 | 泉 結子   | cymy39gqttm  | 545 | 441 | 8fnw4226kd63tv
		// | 1243 | 谷本 楓花 | 2e633gvuk8r  | 563 | 447 | 6f2fkzybgmhxynxp
		// | 1244 | 桑原 楓花 | qdyj7z5vj5   | 523 | 431 | 54f67y4exumtw

		defaultaccounts := []struct {
			account, name, pass string
			order, traded       int
		}{
			{"jz67jt77rpnb", "藍田 奈菜", "7g39gnwr26ze", 501, 424},
			{"2z82n5q", "池野 歩", "2s4s829vm2bg9", 559, 459},
			{"k2vutw", "阿部 俊介", "kgt7e2yv863d5", 557, 449},
			{"yft3f5d5g", "古閑 麻美", "5m99r6vt8qssunb7", 543, 422},
			{"pcsuktmvqn", "川崎 大輝", "fkpcy2amcp9pkmx", 549, 443},
			{"hpnwwt", "吉田 一", "5y62vet3dcepg", 547, 447},
			{"2q5m84je", "相田 大悟", "qme4bak7x3ng", 521, 420},
			{"cymy39gqttm", "泉 結子", "8fnw4226kd63tv", 545, 441},
			{"2e633gvuk8r", "谷本 楓花", "6f2fkzybgmhxynxp", 563, 447},
			{"qdyj7z5vj5", "桑原 楓花", "54f67y4exumtw", 523, 431},
		}
		gd := defaultaccounts[rand.Intn(len(defaultaccounts))]
		gc, err := NewClient(t.appep, gd.account, gd.name, gd.pass, ClientTimeout, RetireTimeout)
		if err != nil {
			return errors.Wrap(err, "create new client failed")
		}
		if err := gc.Signin(ctx); err != nil {
			return errors.Wrapf(err, "Signin(bank:%s,name:%s)", gd.account, gd.name)
		}
		info, err := gc.Info(ctx, 0)
		if err != nil {
			return err
		}
		// TODO: Fix
		if len(info.TradedOrders) < gd.traded {
			return errors.Errorf("GET /info traded_ordersの件数が少ないです user:%d, got: %d, expected: %d", gc.UserID(), len(info.TradedOrders), gd.traded)
		}
		orders, err := gc.GetOrders(ctx)
		if err != nil {
			return err
		}
		if o := len(orders); o < gd.traded {
			return errors.Errorf("GET /orders 件数があいません user:%d, got: %d, expected: %d", gc.UserID(), o, gd.traded)
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
	}

	{
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
	}

	for _, id := range []string{account1, account2} {
		if err := t.isubank.NewBankID(id); err != nil {
			return errors.Wrap(err, "new bank_id failed")
		}
	}

	{
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
		log.Printf("[INFO] signup and signin OK")
	}

	{
		// conflict
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
		log.Printf("[INFO] conflict check OK")
	}

	{
		// お金がない状態でのorder
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
		log.Printf("[INFO] order no money OK")
	}

	// 売り注文は成功する
	{
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
		log.Printf("[INFO] sell order test OK")
	}

	{
		// 注文をして成立させる
		eg := new(errgroup.Group)
		eg.Go(func() error {
			log.Printf("[INFO] run c1 tasks")
			if err := t.isubank.AddCredit(account1, 29000); err != nil {
				return err
			}
			for _, ap := range [][]int64{
				{5, 5105}, // キャンセルされる
				{2, 5100},
				{1, 5099},
				{3, 5104}, // 足りない
				{2, 5106}, // 99とマッチング
			} {
				order, err := c1.AddOrder(ctx, TradeTypeBuy, ap[0], ap[1])
				if err != nil {
					return errors.Wrapf(err, "POST /orders 買い注文に失敗しました [amount:%d, price:%d]", ap[0], ap[1])
				}
				orders, err := c1.GetOrders(ctx)
				if err != nil {
					return err
				}
				if orders[len(orders)-1].ID != order.ID {
					return errors.Errorf("GET /orders 買い注文が反映されていません got: %d, want: %d", orders[len(orders)-1].ID, order.ID)
				}
			}
			log.Printf("[INFO] send order finish")
			err := func() error {
				timeout := time.After(TestTradeTimeout)
				for {
					select {
					case <-timeout:
						return errors.Errorf("成立すべき取引が成立しませんでした(c1)")
					default:
						info, err := c1.Info(ctx, 0)
						if err != nil {
							return err
						}
						if len(info.TradedOrders) == 1 {
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
			if g, w := len(orders), 4; g != w {
				return errors.Errorf("GET /orders 件数があいません [got:%d, want:%d]", g, w)
			}
			if orders[3].Trade == nil {
				return errors.Errorf("GET /orders 成立した注文のtradeが設定されていません")
			}
			bought := orders[3].Trade.Price * 2
			rest, err := t.isubank.GetCredit(account1)
			if err != nil {
				return err
			}
			if rest+bought != 29000 {
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
							if len(fl) < 2 {
								return false, nil
							}
							if fl[0].BuyError.Amount != 1 || fl[0].BuyError.Price != 2000 {
								return false, errors.Errorf("log.buy.errorが正しくありません")
							}
							fl = filterLogs(logs, isulog.TagBuyOrder)
							if len(fl) < 5 {
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
			log.Printf("[INFO] run c2 tasks")
			for _, ap := range [][]int64{
				{6, 5106},
				{2, 5110},
				{3, 5106},
				{7, 5104}, // 足りない
				{1, 5104}, // - 2, 100
				{1, 5104}, // -
			} {
				order, err := c2.AddOrder(ctx, TradeTypeSell, ap[0], ap[1])
				if err != nil {
					return errors.Wrap(err, "POST /orders 売り注文に失敗しました")
				}
				orders, err := c2.GetOrders(ctx)
				if err != nil {
					return err
				}
				if orders[len(orders)-1].ID != order.ID {
					return errors.Errorf("GET /orders 売り注文が反映されていません got: %d, want: %d", orders[len(orders)-1].ID, order.ID)
				}
			}
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
			if g, w := len(orders), 6; g != w {
				return errors.Errorf("GET /orders 件数があいません [got:%d, want:%d]", g, w)
			}
			if orders[4].Trade == nil {
				return errors.Errorf("GET /orders 成立した注文のtradeが設定されていません")
			}
			if orders[5].Trade == nil {
				return errors.Errorf("GET /orders 成立した注文のtradeが設定されていません")
			}
			bought := orders[4].Trade.Price + orders[5].Trade.Price
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
				for {
					select {
					case <-timeout:
						return errors.Errorf("ログが送信されていません(c2)")
					default:
						logs, err := t.isulog.GetUserLogs(c2.UserID())
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
							if len(fl) < 5 {
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
							log.Printf("[INFO] ログチェック OK(c1)")
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

type PostTester struct {
	appep     string
	isulog    *isulog.Isulog
	isubank   *isubank.Isubank
	investors []Investor
}

func (t *PostTester) Run(ctx context.Context) error {
	deadline := time.Now().Add(LogAllowedDelay)
	if len(t.investors) == 0 {
		return errors.Errorf("ユーザーが全滅しています")
	}
	first, latest, random := t.investors[0], t.investors[len(t.investors)-1], t.investors[rand.Intn(len(t.investors))]
	var trade *Trade
	for _, investor := range t.investors {
		for _, order := range investor.Orders() {
			if order.Trade != nil {
				if trade == nil || trade.CreatedAt.Before(order.Trade.CreatedAt) {
					trade = order.Trade
					latest = investor
				}
			}
		}
	}
	if trade == nil {
		return errors.Errorf("取引に成功したユーザーが全滅しています")
	}
	eg := new(errgroup.Group)

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
	for _, inv := range []Investor{first, latest, random} {

		investor := inv
		eg.Go(func() error {
			timeout := time.After(deadline.Sub(time.Now()))
			var credit int64
			for credit != investor.Credit() {
				select {
				case <-timeout:
					return errors.Errorf("銀行残高があいません[user:%d]", investor.UserID())
				default:
					var err error
					credit, err = t.isubank.GetCredit(investor.BankID())
					if err != nil {
						return errors.Wrap(err, "ISUBANK APIとの通信に失敗しました")
					}
					if credit == investor.Credit() {
						log.Printf("[INFO] 残高チェックOK (point1) [user:%d]", investor.UserID())
						break
					}
					if err = investor.FetchOrders(ctx); err != nil {
						return err
					}
					if credit == investor.Credit() {
						log.Printf("[INFO] 残高チェックOK (point2) [user:%d]", investor.UserID())
						break
					}
					time.Sleep(time.Millisecond * 500)
				}
			}
			var buy, sell, buyt, sellt, buyd, selld int
			for _, order := range investor.Orders() {
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
					return errors.Errorf("ログが欠損しています [user:%d]", investor.UserID())
				default:
					logs, err := t.isulog.GetUserLogs(investor.UserID())
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
						log.Printf("[INFO] ユーザーログチェックOK [user:%d]", investor.UserID())
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
