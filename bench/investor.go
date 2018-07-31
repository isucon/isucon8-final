package bench

import "context"

type isuorder struct {
	amount, price int64
	closed        bool
}

type Investor interface {
	// 初期処理を実行するTaskをcontextに追加
	Start(*Worker)

	// Tradeが発生したときに呼ばれる
	Next(*Worker)

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

func (i *investorBase) BuyTask(amount, price int64) Task {
	return NewExecTask(func(ctx context.Context) error {
		if err := i.c.AddBuyOrder(amount, price); err != nil {
			return err
		}
		i.buyorders = append(i.buyorders, isuorder{amount, price, false})
		return nil
	}, PostBuyOrdersScore)
}

func (i *investorBase) SellTask(amount, price int64) Task {
	return NewExecTask(func(ctx context.Context) error {
		if err := i.c.AddSellOrder(amount, price); err != nil {
			return err
		}
		i.sellorders = append(i.sellorders, isuorder{amount, price, false})
	}, PostSellOrdersScore)
}

// あまり考えずに売買する
type RandomInvestor struct {
	*investorBase
}

func NewRandomInvestor(c *Client, credit, isu int64) *RandomInvestor {
	return &RandomInvestor{newInvestorBase(c, credit, isu)}
}

// すぐに取引を成立させるため市場価格を見ずに突っ込む
func (i *RandomInvestor) Start(w *Worker) {
	if i.isu < 3 && i.credit > 100 {
		w.AddTask(i.BuyTask(1, 100))
	} else {
		w.AddTask(i.SellTask(1, 100))
	}
}

func (i *RandomInvestor) Next(w *Worker) {
}
