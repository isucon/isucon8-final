package bench

import (
	"context"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/ken39arg/isucon2018-final/bench/taskworker"
	"github.com/pkg/errors"
)

const (
	OrderCap            = 5
	TradeHistrySize     = 10
	PollingInterval     = 500 * time.Millisecond
	OrderUpdateInterval = 2 * time.Second
)

type Investor interface {
	// 初期処理を実行するTaskを返す
	Start() taskworker.Task

	// tickerで呼ばれる
	Next() taskworker.Task

	BankID() string
	Credit() int64
	Isu() int64
	IsSignin() bool
	IsStarted() bool

	LatestTradePrice() int64
	IsRetired() bool
}

type investorBase struct {
	c                *Client
	defcredit        int64
	credit           int64
	resvedCredit     int64
	defisu           int64
	isu              int64
	resvedIsu        int64
	orders           []*Order
	lowestSellPrice  int64
	highestBuyPrice  int64
	isSignin         bool
	isStarted        bool
	lastCursor       int64
	nextCheck        time.Time
	lastOrder        time.Time
	mux              sync.Mutex
	taskLock         sync.Mutex
	taskStack        []taskworker.Task
	timeoutCount     int
	latestTradePrice int64
}

func newInvestorBase(c *Client, credit, isu int64) *investorBase {
	return &investorBase{
		c:         c,
		defcredit: credit,
		credit:    credit,
		defisu:    isu,
		isu:       isu,
		orders:    make([]*Order, 0, OrderCap),
		taskStack: make([]taskworker.Task, 0, 5),
	}
}

func (i *investorBase) IsRetired() bool {
	return i.c.IsRetired()
}

func (i *investorBase) LatestTradePrice() int64 {
	return i.latestTradePrice
}

func (i *investorBase) pushNextTask(task taskworker.Task) {
	i.taskLock.Lock()
	defer i.taskLock.Unlock()
	i.taskStack = append(i.taskStack, task)
}

func (i *investorBase) pushOrder(o *Order) {
	if o == nil {
		log.Printf("[WARN] push order is null")
		return
	}
	//log.Printf("[DEBUG] pushOrder [id:%d]", o.ID)
	i.orders = append(i.orders, o)
}

func (i *investorBase) removeOrder(id int64) *Order {
	//log.Printf("[DEBUG] removeOrder [id:%d]", id)
	newl := make([]*Order, 0, cap(i.orders))
	var removed *Order
	for _, o := range i.orders {
		if o == nil {
			continue
		}
		if o.ID != id {
			newl = append(newl, o)
		} else {
			removed = o
		}
	}
	i.orders = newl
	return removed
}

func (i *investorBase) hasOrder(id int64) bool {
	for _, o := range i.orders {
		if o.ID == id {
			return true
		}
	}
	return false
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

func (i *investorBase) IsSignin() bool {
	return i.isSignin
}

func (i *investorBase) IsStarted() bool {
	return i.isStarted
}

func (i *investorBase) Top() taskworker.Task {
	return taskworker.NewExecTask(func(_ context.Context) error {
		return i.c.Top()
	}, GetTopScore)
}

func (i *investorBase) Signup() taskworker.Task {
	return taskworker.NewScoreTask(func(_ context.Context) (int64, error) {
		time.Sleep(time.Millisecond * time.Duration(rand.Int63n(100)))
		i.mux.Lock()
		defer i.mux.Unlock()
		if i.IsSignin() {
			return 0, nil
		}
		if err := i.c.Signup(); err != nil {
			if strings.Index(err.Error(), "bank_id already exists") > -1 {
				return SignupScore, nil
			}
			return 0, err
		}
		return SignupScore, nil
	})
}

func (i *investorBase) Signin() taskworker.Task {
	return taskworker.NewExecTask(func(_ context.Context) error {
		time.Sleep(time.Millisecond * time.Duration(rand.Int63n(100)))
		if err := i.c.Signin(); err != nil {
			return err
		}
		i.isSignin = true
		return nil

	}, SigninScore)
}

func (i *investorBase) Info() taskworker.Task {
	return taskworker.NewScoreTask(func(ctx context.Context) (int64, error) {
		i.mux.Lock()
		defer i.mux.Unlock()
		if i.IsRetired() {
			// already retired
			return 0, nil
		}
		now := time.Now()
		if now.Before(i.nextCheck) {
			//log.Printf("[INFO] skip info() next: %s now: %s", i.nextCheck, now)
			return 0, nil
		}
		info, err := i.c.Info(i.lastCursor)
		if err != nil {
			return 0, err
		}
		i.nextCheck = time.Now().Add(PollingInterval)
		var score int64 = GetInfoScore
		// TODO CandlestickData のチェック
		i.lowestSellPrice = info.LowestSellPrice
		i.highestBuyPrice = info.HighestBuyPrice
		i.lastCursor = info.Cursor
		if l := len(info.ChartByHour); l > 0 {
			i.latestTradePrice = info.ChartByHour[l-1].Close
		}

		for _, order := range info.TradedOrders {
			o := i.removeOrder(order.ID)
			if o == nil {
				if i.timeoutCount > 0 {
					continue
				}
				return score, errors.Errorf("received `traded_order` that is not mine, or already closed. [%d]", order.ID)
			}
			if o.Type != order.Type {
				return score, errors.Errorf("received `traded_order` that is not match order type. [%d]", order.ID)
			}
			if o.Amount != order.Amount {
				return score, errors.Errorf("received `traded_order` that is not match order amount. [%d]", order.ID)
			}
			if o.Price != order.Price {
				return score, errors.Errorf("received `traded_order` that is not match order price. [%d]", order.ID)
			}
			if order.Trade != nil {
				switch order.Type {
				case TradeTypeSell:
					// 売り注文成立
					if order.Price > order.Trade.Price {
						return score, errors.Errorf("traded price for sell order is cheaper than order price. [%d]", order.ID)
					}
					i.isu -= order.Amount
					i.credit += order.Trade.Price * order.Amount
					score += TradeSuccessScore
				case TradeTypeBuy:
					// 買い注文成立
					if order.Price < order.Trade.Price {
						return score, errors.Errorf("traded price for sell order is higher than order price. [%d]", order.ID)
					}
					i.isu += order.Amount
					i.credit -= order.Trade.Price * order.Amount
					score += TradeSuccessScore
				}
			}
		}
		return score, nil
	})
}

func (i *investorBase) UpdateOrders() taskworker.Task {
	return taskworker.NewExecTask(func(_ context.Context) error {
		i.mux.Lock()
		defer i.mux.Unlock()
		orders, err := i.c.GetOrders()
		if err != nil {
			return err
		}
		if g, w := len(orders), len(i.orders); g < w {
			return errors.Errorf("few orders. got:%d, want:%d", g, w)
		}
		if i.timeoutCount == 0 && len(orders) > 0 && len(i.orders) > 0 && orders[len(orders)-1].ID != i.orders[len(i.orders)-1].ID {
			return errors.Errorf("orders is not last ordered. got:[len:%d,lastid:%d], want:[len:%d,lastid:%d]", len(orders), orders[len(orders)-1].ID, len(i.orders), i.orders[len(i.orders)-1].ID)
		}
		var resvedCredit, resvedIsu, tradedIsu, tradedCredit int64
		for _, order := range orders {
			switch {
			case order.Trade != nil && order.Type == TradeTypeSell:
				// 成立済み 売り注文
				tradedIsu -= order.Amount
				tradedCredit += order.Amount * order.Trade.Price
			case order.Trade != nil && order.Type == TradeTypeBuy:
				// 成立済み 買い注文
				tradedIsu += order.Amount
				tradedCredit -= order.Amount * order.Trade.Price
			case order.Type == TradeTypeSell:
				// 売り注文
				resvedIsu += order.Amount
				if !i.hasOrder(order.ID) {
					if i.timeoutCount == 0 {
						return errors.Errorf("fined removed order [orderID:%d]", order.ID)
						//if len(i.orders) < cap(i.orders) {
						//	i.pushOrder(&order)
						//	//i.timeoutCount--
						//}
					}
				}
			case order.Type == TradeTypeBuy:
				// 買い注文
				resvedCredit += order.Amount * order.Price
				if !i.hasOrder(order.ID) {
					if i.timeoutCount == 0 {
						return errors.Errorf("fined removed order [orderID:%d]", order.ID)
						//if len(i.orders) < cap(i.orders) {
						//	i.pushOrder(&order)
						//	//i.timeoutCount--
						//}
					}
				}
			}
		}
		i.resvedIsu = resvedIsu
		i.resvedCredit = resvedCredit
		if current := i.defcredit + tradedCredit; i.credit != current {
			log.Printf("[WARN] credit mismach got %d want %d", i.credit, current)
			i.credit = current
		}
		if current := i.defisu + tradedIsu; i.isu != current {
			log.Printf("[WARN] isu mismach got %d want %d", i.isu, current)
			i.isu = current
		}
		return nil
	}, GetOrdersScore)
}

func (i *investorBase) AddOrder(ot string, amount, price int64) taskworker.Task {
	return taskworker.NewExecTask(func(ctx context.Context) error {
		i.mux.Lock()
		defer i.mux.Unlock()
		order, err := i.c.AddOrder(ot, amount, price)
		if err != nil {
			// 残高不足はOKとする
			if strings.Index(err.Error(), "銀行残高が足りません") > -1 {
				return nil
			}
			return err
		}
		i.pushOrder(order)
		i.lastOrder = time.Now()
		return nil
	}, PostOrdersScore)
}

func (i *investorBase) RemoveOrder(order *Order) taskworker.Task {
	return taskworker.NewScoreTask(func(ctx context.Context) (int64, error) {
		i.mux.Lock()
		defer i.mux.Unlock()
		if !i.hasOrder(order.ID) {
			return 0, nil
		}
		if err := i.c.DeleteOrders(order.ID); err != nil {
			if er, ok := err.(*ErrorWithStatus); ok && er.StatusCode == 404 {
				// 404エラーはしょうがないのでerrにはしないが加点しない
				log.Printf("[INFO] delete 404 %s", er)
				return 0, nil
			}
			return 0, err
		}
		i.removeOrder(order.ID)
		return DeleteOrdersScore, nil
	})
}

func (i *investorBase) Start() taskworker.Task {
	i.isStarted = true
	task := taskworker.NewSerialTask(6)
	task.Add(i.Top())
	task.Add(i.Info())
	task.Add(i.Signup())
	task.Add(i.Signin())
	task.Add(i.UpdateOrders())
	return task
}

func (i *investorBase) Next() taskworker.Task {
	i.taskLock.Lock()
	defer i.taskLock.Unlock()
	if i.IsRetired() {
		return nil
	}
	task := taskworker.NewSerialTask(2 + len(i.taskStack))
	task.Add(i.Info())
	for _, t := range i.taskStack {
		task.Add(t)
	}
	i.taskStack = i.taskStack[:0]
	return task
}

// あまり考えずに売買する
// 特徴:
//  - isuがunitamount以上余っていたら売りたい
//  - unitpriceよりも高値で売れそうなら売りたい
//  - unitpriceよりも安値で買えそうならunitamountの範囲で買いたい
//  - 取引価格がunitpriceとかけ離れたら深く考えずにunitpriceを見直す
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

func (i *RandomInvestor) Start() taskworker.Task {
	task := i.investorBase.Start()
	if t, ok := task.(*taskworker.SerialTask); ok {
		t.Add(i.FixNextTask())
		return t
	}
	return task
}

func (i *RandomInvestor) Next() taskworker.Task {
	if i.IsRetired() {
		return nil
	}
	task := i.investorBase.Next()
	if t, ok := task.(*taskworker.SerialTask); ok {
		t.Add(i.FixNextTask())
		return t
	}
	return task
}

func (i *RandomInvestor) FixNextTask() taskworker.Task {
	return taskworker.NewExecTask(func(_ context.Context) error {
		if task := i.UpdateOrderTask(); task != nil {
			i.pushNextTask(task)
			i.pushNextTask(i.UpdateOrders())
		}
		return nil
	}, 0)
}

func (i *RandomInvestor) UpdateOrderTask() taskworker.Task {
	i.mux.Lock()
	defer i.mux.Unlock()
	if i.IsRetired() {
		return nil
	}
	now := time.Now()
	update := len(i.orders) == 0 || i.lastOrder.Add(OrderUpdateInterval).After(now)

	if !update {
		//log.Printf("skip update order last:%s, now:%s", i.lastOrder, now)
		return nil
	}
	logicalCredit := i.credit - i.resvedCredit
	logicalIsu := i.isu - i.resvedIsu
	switch {
	case len(i.orders) == OrderCap:
		// orderを一個リリースする
		var o *Order
		var df int64
		for _, order := range i.orders {
			var mdiff int64
			if order.Type == TradeTypeSell {
				mdiff = order.Price - i.highestBuyPrice
			} else {
				mdiff = i.lowestSellPrice - order.Price
			}
			if o == nil || df < mdiff {
				o = order
				df = mdiff
			}
		}
		return i.RemoveOrder(o)
	case len(i.orders) == 0 && logicalIsu > i.unitamount:
		// 初注文は絶対する
		return i.AddOrder(TradeTypeSell, i.unitamount, i.unitprice)
	case len(i.orders) == 0:
		// 初注文は絶対する
		return i.AddOrder(TradeTypeBuy, i.unitamount, i.unitprice)
	case i.lowestSellPrice > 0 && i.lowestSellPrice < i.unitprice && i.lowestSellPrice <= logicalCredit:
		// 最安売値が設定値より安いので買いたい
		amount := rand.Int63n(logicalCredit/i.lowestSellPrice) + 1
		if i.unitamount < amount {
			amount = i.unitamount
		}
		return i.AddOrder(TradeTypeBuy, amount, i.lowestSellPrice)
	case i.highestBuyPrice > 0 && i.highestBuyPrice > i.unitprice && logicalIsu > 0:
		// 最高買値が設定値より高いので売りたい
		amount := rand.Int63n(logicalIsu) + 1
		if i.unitamount < amount {
			amount = i.unitamount
		}
		return i.AddOrder(TradeTypeSell, amount, i.highestBuyPrice)
	case logicalIsu > i.unitamount:
		// 椅子をたくさん持っていて現在価格が希望外のときは少し妥協して売りに行く
		price := (i.lowestSellPrice + i.unitprice) / 2
		if i.lowestSellPrice == 0 {
			price = i.unitprice
		}
		amount := rand.Int63n(i.unitamount) + 1
		return i.AddOrder(TradeTypeSell, amount, price)
	case logicalCredit > (i.highestBuyPrice+i.unitprice)/2:
		// 金があるので妥協価格で買い注文を入れる
		price := (i.highestBuyPrice + i.unitprice) / 2
		if i.highestBuyPrice == 0 {
			price = i.unitprice
		}
		amount := rand.Int63n(i.unitamount) + 1
		return i.AddOrder(TradeTypeBuy, amount, price)
	default:
		// 椅子評価額の見直し
		if latestPrice := i.LatestTradePrice(); latestPrice > 0 {
			i.unitprice = (latestPrice + i.unitprice) / 2
		}
	}
	return nil
}
