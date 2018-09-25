package bench

import (
	"fmt"
	"time"

	"github.com/ken39arg/isucon2018-final/bench/isubank"
	"github.com/ken39arg/isucon2018-final/bench/isulog"
	"github.com/pkg/errors"
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

	c1, err := NewClient(t.appep, account1, "鈴木 明", "1234567890abc", ClientTimeout, RetireTimeout)
	if err != nil {
		return errors.Wrap(err, "create new client failed")
	}
	c2, err := NewClient(t.appep, account2, "トニー モリス", "234567890abcd", ClientTimeout, RetireTimeout)
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

	for _, c := range []*Client{c1, c2} {
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
	}

	{
		// お金がない状態でのorder
		_, err := c1.AddOrder(TradeTypeBuy, 1, 2000)
		if err == nil {
			return errors.New("POST /orders 銀行に残高が足りない買い注文に成功しました")
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
		o, err := c1.AddOrder(TradeTypeSell, 1, 2000)
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
