package bench

import (
	"context"
	"math/rand"
	"strings"

	"github.com/pkg/errors"
)

type Investor interface {
	// 初期処理を実行するTaskを返す
	Start() Task

	// Tradeが発生したときに呼ばれる
	Next([]Trade) Task

	BankID() string
	Credit() int64
	Isu() int64
}

type investorBase struct {
	c           *Client
	credit      int64
	isu         int64
	sellorder   int
	buyorder    int
	buyhistory  map[int64]Order
	sellhistory map[int64]Order
}

func newInvestorBase(c *Client, credit, isu int64) *investorBase {
	return &investorBase{
		c:           c,
		credit:      credit,
		isu:         isu,
		buyhistory:  map[int64]Order{},
		sellhistory: map[int64]Order{},
	}
}

func (i *investorBase) BankID() string {
	return i.c.bankid
}

func (i *investorBase) Credit() int64 {
	return i.credit
}

func (i *investorBase) Isu() int64 {
	return i.isu
}

func (i *investorBase) Signup() Task {
	return NewExecTask(func(_ context.Context) error {
		return i.c.Signup()
	}, SignupScore)
}

func (i *investorBase) Signin() Task {
	return NewExecTask(func(_ context.Context) error {
		return i.c.Signin()
	}, SigninScore)
}

func (i *investorBase) BuyOrder(amount, price int64) Task {
	return NewExecTask(func(ctx context.Context) error {
		if err := i.c.AddBuyOrder(amount, price); err != nil {
			if strings.Index(err.Error(), "銀行残高が足りません") > -1 {
				return ErrNoScore
			}
			return err
		}
		i.buyorder++
		return nil
	}, PostBuyOrdersScore)
}

func (i *investorBase) SellOrder(amount, price int64) Task {
	return NewExecTask(func(ctx context.Context) error {
		if err := i.c.AddSellOrder(amount, price); err != nil {
			return err
		}
		i.sellorder++
		return nil
	}, PostSellOrdersScore)
}

func (i *investorBase) UpdateBuyOrders() Task {
	return NewExecTask(func(ctx context.Context) error {
		if i.buyorder == 0 {
			return ErrNoScore
		}
		orders, err := i.c.BuyOrders()
		if err != nil {
			return err
		}
		for _, order := range orders {
			if order.ClosedAt == nil {
				continue
			}
			if _, ok := i.buyhistory[order.ID]; ok {
				continue
			}
			i.buyhistory[order.ID] = order
			i.buyorder--
			if order.Trade != nil {
				// 買い取りは安くなければだめ
				if order.Price < order.Trade.Price {
					return errors.Errorf("買い注文の指値より高値で取引されています. order:%d", order.ID)
				}
				i.credit -= order.Trade.Price * order.Amount
				i.isu += order.Amount
			}
		}
		return nil
	}, GetBuyOrdersScore)
}

func (i *investorBase) UpdateSellOrders() Task {
	return NewExecTask(func(ctx context.Context) error {
		if i.sellorder == 0 {
			return ErrNoScore
		}
		orders, err := i.c.SellOrders()
		if err != nil {
			return err
		}
		for _, order := range orders {
			if order.ClosedAt == nil {
				continue
			}
			if _, ok := i.sellhistory[order.ID]; ok {
				continue
			}
			i.sellhistory[order.ID] = order
			i.sellorder--
			if order.Trade != nil {
				// 売却は高くなければだめ
				if order.Price > order.Trade.Price {
					return errors.Errorf("売り注文の指値より安値で取引されています. order:%d", order.ID)
				}
				i.credit += order.Trade.Price * order.Amount
				i.isu -= order.Amount
			}
		}
		return nil
	}, GetSellOrdersScore)
}

// あまり考えずに売買する
type RandomInvestor struct {
	*investorBase
	unitamount int64
	unitprice  int64
}

func NewRandomInvestor(c *Client, credit, isu, unitamount, unitprice int64) *RandomInvestor {
	return &RandomInvestor{
		investorBase: newInvestorBase(c, credit, isu),
		unitamount:   unitamount,
		unitprice:    unitprice,
	}
}

func (i *RandomInvestor) Start() Task {
	task := NewListTask(3)
	task.Add(i.Signup())
	task.Add(i.Signin())

	if i.isu < i.unitamount && i.credit > i.unitprice {
		task.Add(i.BuyOrder(rand.Int63n(i.unitamount)+1, i.unitprice))
	} else {
		task.Add(i.SellOrder(rand.Int63n(i.unitamount)+1, i.unitprice))
	}

	return task
}

func (i *RandomInvestor) Next(trades []Trade) Task {
	task := NewListTask(3)
	task.Add(i.UpdateSellOrders())
	task.Add(i.UpdateBuyOrders())
	r := rand.Intn(10)
	amount := rand.Int63n(i.unitamount) + 1
	price := rand.Int63n(i.unitprice/2) - (i.unitprice / 4) + trades[0].Price
	if r < 2 {
		// このターンは何もしない
	} else if r < 6 {
		// このターンは買う
		task.Add(NewExecTask(func(ctx context.Context) error {
			if i.credit < price*amount {
				// 資金がない
				return ErrNoScore
			}
			if err := i.c.AddBuyOrder(amount, price); err != nil {
				if strings.Index(err.Error(), "銀行残高が足りません") > -1 {
					return ErrNoScore
				}
				return err
			}
			i.buyorder++
			return nil
		}, PostBuyOrdersScore))
	} else {
		// このターンは売る
		task.Add(NewExecTask(func(ctx context.Context) error {
			if i.isu < amount {
				amount = i.isu
			}
			if amount <= 0 {
				// 売る椅子がない
				return ErrNoScore
			}
			if err := i.c.AddSellOrder(amount, price); err != nil {
				return err
			}
			i.sellorder++
			return nil
		}, PostSellOrdersScore))
	}
	return task
}
