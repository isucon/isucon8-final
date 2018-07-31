package bench

import (
	"context"
	"math/rand"
)

type isuorder struct {
	amount, price int64
	closed        bool
}

type Investor interface {
	// 初期処理を実行するTaskをcontextに追加
	Start(*Worker)

	// Tradeが発生したときに呼ばれる
	Next(*Worker, []Trade)

	BankID() string
}

type investorBase struct {
	c          *Client
	credit     int64
	isu        int64
	buyorders  []isuorder
	sellorders []isuorder
}

func newInvestorBase(c *Client, credit, isu int64) *investorBase {
	return &investorBase{c, credit, isu, make([]isuorder, 0, 5), make([]isuorder, 0, 5)}
}

func (i *investorBase) BankID() string {
	return i.c.bankid
}

func (i *investorBase) Signup() func(context.Context) error {
	return func(_ context.Context) error {
		return i.c.Signup()
	}
}

func (i *investorBase) Signin() func(context.Context) error {
	return func(_ context.Context) error {
		return i.c.Signin()
	}
}

func (i *investorBase) BuyOrder(amount, price int64) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := i.c.AddBuyOrder(amount, price); err != nil {
			return err
		}
		i.buyorders = append(i.buyorders, isuorder{amount, price, false})
		return nil
	}
}

func (i *investorBase) SellOrder(amount, price int64) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := i.c.AddSellOrder(amount, price); err != nil {
			return err
		}
		i.sellorders = append(i.sellorders, isuorder{amount, price, false})
	}
}

// あまり考えずに売買する
type RandomInvestor struct {
	*investorBase
	unitamount int64
	unitprice  int64
}

func NewRandomInvestor(c *Client, credit, isu, unitamount, unitprice int64) *RandomInvestor {
	return &RandomInvestor{newInvestorBase(c, credit, isu), unitamount, unitprice}
}

func (i *RandomInvestor) Start(w *Worker) {
	task := NewListTask(3)
	task.Add(i.Signup(), SignupScore)
	task.Add(i.Signin(), SigninScore)

	// すぐに取引を成立させるため市場価格を見ずに突っ込む
	if i.isu < i.amount && i.credit > i.unitprice {
		task.Add(i.BuyOrder(rand.Int63n(i.unitamount)+1, i.unitprice), PostBuyOrdersScore)
	} else {
		task.Add(i.SellOrder(i.unitamount, i.unitprice), PostSellOrdersScore)
	}
	w.AddTask(task)
}

func (i *RandomInvestor) Next(w *Worker, trades []Trade) {
	lastPrice := trades[0].Price
	state := 0
	if len(trades) > 1 {
		if trades[1].Price < lastPrice {
			state = 1
		} else if trades[1].Price > lastPrice {
			state = -1
		}
	}
}
