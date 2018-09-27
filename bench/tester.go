package bench

import (
	"fmt"
	"log"
	"time"

	"github.com/ken39arg/isucon2018-final/bench/isubank"
	"github.com/ken39arg/isucon2018-final/bench/isulog"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type tester struct {
	appep   string
	isulog  *isulog.Isulog
	isubank *isubank.Isubank
}

func newtester(a string, l *isulog.Isulog, b *isubank.Isubank) *tester {
	return &tester{a, l, b}
}

type PreTester struct {
	*tester
}

func NewPreTester(a string, l *isulog.Isulog, b *isubank.Isubank) *PreTester {
	return &PreTester{
		tester: newtester(a, l, b),
	}
}

func (t *PreTester) Run() error {
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
	if err := c2.Top(); err != nil {
		return err
	}

	{
		// 非ログイン /info
		info, err := c2.Info(0)
		if err != nil {
			return err
		}

		if info.TradedOrders != nil && len(info.TradedOrders) > 0 {
			return errors.Errorf("GET /info ゲストユーザーのtraded_ordersが設定されています")
		}
		// TODO 初期データを入れてテスト
	}
	{
		// アカウントがない
		err := c1.Signin()
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
		// BANK IDが存在しない
		err := c1.Signup()
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
				if err := c.Top(); err != nil {
					return err
				}
				if _, err := c.Info(0); err != nil {
					return err
				}
				if err := c.Signup(); err != nil {
					return err
				}
				if err := c.Signin(); err != nil {
					return err
				}
				if _, err := c.GetOrders(); err != nil {
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
		err = c1x.Signup()
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
		order, err := c1.AddOrder(TradeTypeBuy, 1, 2000)
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
		o, err := c1.AddOrder(TradeTypeSell, 1, 6000)
		if err != nil {
			return err
		}
		orders, err := c1.GetOrders()
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

		if err = c1.DeleteOrders(o.ID); err != nil {
			return err
		}
		orders, err = c1.GetOrders()
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
				order, err := c1.AddOrder(TradeTypeBuy, ap[0], ap[1])
				if err != nil {
					return errors.Wrapf(err, "POST /orders 買い注文に失敗しました [amount:%d, price:%d]", ap[0], ap[1])
				}
				orders, err := c1.GetOrders()
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
				next := make(chan bool, 1)
				defer close(next)
				next <- true
				for {
					select {
					case <-timeout:
						return errors.Errorf("成立すべき取引が成立しませんでした(c1)")
					case <-next:
						info, err := c1.Info(0)
						if err != nil {
							return err
						}
						if len(info.TradedOrders) == 1 {
							return nil
						}
						time.Sleep(PollingInterval)
						next <- true
					}
				}
			}()
			if err != nil {
				return err
			}
			log.Printf("[INFO] trade sucess OK(c1)")

			orders, err := c1.GetOrders()
			if err != nil {
				return err
			}
			if g, w := len(orders), 4; g != w {
				return errors.Errorf("GET /orders 件数があいません [got:%d, want:%d]", g, w)
			}
			if orders[3].Trade == nil {
				return errors.Errorf("GET /orders 成立した注文のtradeが設定されていません")
			}
			buyed := orders[3].Trade.Price * 2
			rest, err := t.isubank.GetCredit(account1)
			if err != nil {
				return err
			}
			if rest+buyed != 29000 {
				return errors.Errorf("銀行残高があいません [%d]", rest)
			}
			log.Printf("[INFO] 残高チェック OK(c1)")

			return func() error {
				timeout := time.After(LogAllowedDelay)
				next := make(chan bool, 1)
				defer close(next)
				next <- true
				for {
					select {
					case <-timeout:
						return errors.Errorf("ログが送信されていません(c1)")
					case <-next:
						logs, err := t.isulog.GetUserLogs(c1.UserID())
						if err != nil {
							return errors.Wrap(err, "isulog get user logs failed")
						}
						ok, err := func() (bool, error) {
							var fl []*isulog.Log
							fl = filetrLogs(logs, isulog.TagSignup)
							if len(fl) == 0 {
								return false, nil
							}
							if fl[0].Signup.Name != name1 {
								return false, errors.Errorf("log.signup のnameが正しくありません")
							}
							if fl[0].Signup.BankID != account1 {
								return false, errors.Errorf("log.signup のbank_idが正しくありません")
							}
							fl = filetrLogs(logs, isulog.TagSignin)
							if len(fl) == 0 {
								return false, nil
							}
							fl = filetrLogs(logs, isulog.TagBuyError)
							if len(fl) < 2 {
								return false, nil
							}
							if fl[0].BuyError.Amount != 1 || fl[0].BuyError.Price != 2000 {
								return false, errors.Errorf("log.buy.errorが正しくありません")
							}
							fl = filetrLogs(logs, isulog.TagBuyOrder)
							if len(fl) < 5 {
								return false, nil
							}
							fl = filetrLogs(logs, isulog.TagBuyTrade)
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
						next <- true
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
				order, err := c2.AddOrder(TradeTypeSell, ap[0], ap[1])
				if err != nil {
					return errors.Wrap(err, "POST /orders 売り注文に失敗しました")
				}
				orders, err := c2.GetOrders()
				if err != nil {
					return err
				}
				if orders[len(orders)-1].ID != order.ID {
					return errors.Errorf("GET /orders 売り注文が反映されていません got: %d, want: %d", orders[len(orders)-1].ID, order.ID)
				}
			}
			err := func() error {
				timeout := time.After(TestTradeTimeout)
				next := make(chan bool, 1)
				defer close(next)
				next <- true
				for {
					select {
					case <-timeout:
						return errors.Errorf("成立すべき取引が成立しませんでした(c2)")
					case <-next:
						info, err := c2.Info(0)
						if err != nil {
							return err
						}
						if len(info.TradedOrders) == 2 {
							return nil
						}
						time.Sleep(PollingInterval)
						next <- true
					}
				}
			}()
			if err != nil {
				return err
			}
			log.Printf("[INFO] trade sucess OK(c2)")

			orders, err := c2.GetOrders()
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
			buyed := orders[4].Trade.Price + orders[5].Trade.Price
			rest, err := t.isubank.GetCredit(account2)
			if err != nil {
				return err
			}
			if rest != buyed {
				return errors.Errorf("銀行残高があいません [%d]", rest)
			}
			log.Printf("[INFO] 残高チェック OK(c2)")

			return func() error {
				timeout := time.After(LogAllowedDelay)
				next := make(chan bool, 1)
				defer close(next)
				next <- true
				for {
					select {
					case <-timeout:
						return errors.Errorf("ログが送信されていません(c2)")
					case <-next:
						logs, err := t.isulog.GetUserLogs(c2.UserID())
						if err != nil {
							return errors.Wrap(err, "isulog get user logs failed")
						}
						ok, err := func() (bool, error) {
							var fl []*isulog.Log
							fl = filetrLogs(logs, isulog.TagSignup)
							if len(fl) == 0 {
								return false, nil
							}
							if fl[0].Signup.Name != name2 {
								return false, errors.Errorf("log.signup のnameが正しくありません")
							}
							if fl[0].Signup.BankID != account2 {
								return false, errors.Errorf("log.signup のbank_idが正しくありません")
							}
							fl = filetrLogs(logs, isulog.TagSignin)
							if len(fl) == 0 {
								return false, nil
							}
							fl = filetrLogs(logs, isulog.TagSellOrder)
							if len(fl) < 5 {
								return false, nil
							}
							fl = filetrLogs(logs, isulog.TagSellTrade)
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
						next <- true
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
	*tester
}

func NewPostTester(a string, l *isulog.Isulog, b *isubank.Isubank) *PostTester {
	return &PostTester{
		tester: newtester(a, l, b),
	}
}

func (t *PostTester) Run() error {
	return nil
}

func filetrLogs(logs []*isulog.Log, tag string) []*isulog.Log {
	ret := make([]*isulog.Log, 0, len(logs))
	for _, l := range logs {
		if l.Tag == tag {
			ret = append(ret, l)
		}
	}
	return ret
}
