package bench

import (
	"context"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type ScoreType int

const (
	ScoreTypeGetTop ScoreType = 1 + iota
	ScoreTypeSignup
	ScoreTypeSignin
	ScoreTypeGetInfo
	ScoreTypePostOrders
	ScoreTypeGetOrders
	ScoreTypeDeleteOrders
	ScoreTypeTradeSuccess
)

type ScoreMsg struct {
	st  ScoreType
	err error
	sns bool
}

func (sm ScoreMsg) Score() int64 {
	if sm.err != nil {
		return 0
	}
	switch sm.st {
	case ScoreTypeGetTop:
		return GetTopScore
	case ScoreTypeSignup:
		return SignupScore
	case ScoreTypeSignin:
		return SigninScore
	case ScoreTypeGetInfo:
		return GetInfoScore
	case ScoreTypeGetOrders:
		return GetOrdersScore
	case ScoreTypePostOrders:
		return PostOrdersScore
	case ScoreTypeDeleteOrders:
		return DeleteOrdersScore
	case ScoreTypeTradeSuccess:
		return TradeSuccessScore
	default:
		log.Printf("[WARN] not defined score [%d]", sm.st)
		return 0
	}
}

type Scenario interface {
	Run(context.Context, chan ScoreMsg) error
	Stop(context.Context) error
	IsSignin() bool
	IsRetired() bool
}

type NormalScenario struct {
	c         *Client
	isSignin  bool
	isRetired bool

	lowestSellPrice  int64
	highestBuyPrice  int64
	latestTradePrice int64
	enableShare      bool
	orders           []*Order

	unitIsu        int64
	defaultIsu     int64
	defaultCredit  int64
	reservedIsu    int64
	reservedCredit int64
	currentIsu     int64
	currentCredit  int64
}

func (s *NormalScenario) IsSignin() bool {
	return s.isSignin
}

func (s *NormalScenario) IsRetired() bool {
	return s.isRetired
}

func (s *NormalScenario) waitingOrders() int {
	c := 0
	for _, o := range s.orders {
		if o.ClosedAt == nil {
			c++
		}
	}
	return c
}

func (s *NormalScenario) Start(ctx context.Context, smchan chan ScoreMsg) error {
	err := s.c.Top(ctx)
	smchan <- ScoreMsg{st: ScoreTypeGetTop, err: err}
	if err != nil {
		return errors.Wrap(err, "トップページを表示できません")
	}

	_, _, err = s.fetchInfo(ctx, 0)
	smchan <- ScoreMsg{st: ScoreTypeGetInfo, err: err}
	if err != nil {
		return errors.Wrap(err, "トップページを表示できません")
	}

	err = s.c.Signup(ctx)
	smchan <- ScoreMsg{st: ScoreTypeSignup, err: err}
	if err != nil {
		return errors.Wrap(err, "アカウントを作成できませんでした")
	}

	err = s.c.Signin(ctx)
	smchan <- ScoreMsg{st: ScoreTypeSignin, err: err}
	if err != nil {
		return errors.Wrap(err, "ログインできませんでした")
	}
	s.isSignin = true

	go s.runInfoLoop(ctx, smchan)

	return nil
}

func (s *NormalScenario) runInfoLoop(ctx context.Context, smchan chan ScoreMsg) {
	var (
		cursor        int64
		lastOrderTime time.Time
		actionLock    sync.Mutex
		runningAction bool
	)
	tryLock := func() bool {
		actionLock.Lock()
		defer actionLock.Unlock()
		if runningAction {
			return false
		}
		runningAction = true
		return true
	}
	unlock := func() {
		actionLock.Lock()
		runningAction = false
		actionLock.Unlock()
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if s.c.IsRetired() {
				return
			}
			next, traded, err := s.fetchInfo(ctx, cursor)
			smchan <- ScoreMsg{st: ScoreTypeGetInfo, err: err}
			if next > 0 {
				cursor = next
			}
			switch {
			case traded:
				// トレードが成立したっぽいときはorderを再取得し、トレードが成功していたら注文し直す
				go func() {
					if !tryLock() {
						return
					}
					defer unlock()
					for {
						tradedOrders, err := s.FetchOrders(ctx)
						smchan <- ScoreMsg{st: ScoreTypeGetOrders, err: err}
						if err != nil {
							return
						}
						if len(tradedOrders) == 0 {
							return
						}
						for range tradedOrders {
							smchan <- ScoreMsg{st: ScoreTypeTradeSuccess}
						}
						time.Sleep(100 * time.Millisecond)
						st, err := s.tryTrade(ctx)
						if st == 0 {
							return
						}
						smchan <- ScoreMsg{st: st, err: err}
						lastOrderTime = time.Now()
						if err != nil {
							return
						}
					}
				}()
			case s.lowestSellPrice < s.highestBuyPrice:
				// 取引成立状態の場合は注文しない
			case lastOrderTime.Add(OrderUpdateInterval + time.Duration(s.waitingOrders()*500)*time.Millisecond).Before(time.Now()):
				// 前回注文してから時間が経過していない場合は注文しない
			default:
				// tradedOrders受信時とは順番が逆になる
				go func() {
					if !tryLock() {
						return
					}
					defer unlock()
					for {
						st, err := s.tryTrade(ctx)
						if st == 0 {
							return
						}
						smchan <- ScoreMsg{st: st, err: err}
						lastOrderTime = time.Now()
						if err != nil {
							return
						}
						tradedOrders, err := s.FetchOrders(ctx)
						smchan <- ScoreMsg{st: ScoreTypeGetOrders, err: err}
						if err != nil {
							return
						}
						if len(tradedOrders) == 0 {
							return
						}
						for range tradedOrders {
							smchan <- ScoreMsg{st: ScoreTypeTradeSuccess}
						}
						time.Sleep(100 * time.Millisecond)
					}
				}()
			}
		}
	}
}

func (s *NormalScenario) fetchInfo(ctx context.Context, cursor int64) (int64, bool, error) {
	var traded bool
	info, err := s.c.Info(ctx, cursor)
	if err != nil {
		return cursor, traded, err
	}
	s.lowestSellPrice = info.LowestSellPrice
	s.highestBuyPrice = info.HighestBuyPrice
	s.enableShare = info.EnableShare
	if l := len(info.ChartByHour); l > 0 {
		s.latestTradePrice = info.ChartByHour[l-1].Close
	}

	if info.TradedOrders != nil && len(info.TradedOrders) > 0 {
		// トレードが成立しているようだ
		for _, order := range info.TradedOrders {
			if order.Trade == nil {
				return info.Cursor, traded, errors.Errorf("GET /info traded_order.trade is null")
			}
			for _, mo := range s.orders {
				if mo.ID == order.ID && mo.TradeID == 0 {
					traded = true
				}
			}
		}
	}

	return info.Cursor, traded, nil
}

func (s *NormalScenario) FetchOrders(ctx context.Context) ([]*Order, error) {
	orders, err := s.c.GetOrders(ctx)
	if err != nil {
		return nil, err
	}
	if len(s.orders) > 0 {
		var lo *Order
		// cancelされていない最後の注文
		for j := len(s.orders) - 1; j >= 0; j-- {
			if s.orders[j].ClosedAt == nil {
				lo = s.orders[j]
				break
			}
		}
		if lo != nil && lo.Type == TradeTypeSell {
			// 買い注文は即cancelされる可能性があるので調べない
			var ok bool
			for _, glo := range orders {
				if lo.ID == glo.ID {
					ok = true
					break
				}
			}
			if !ok {
				return nil, errors.Errorf("GET /orders 注文内容が反映されていません id:%d", lo.ID)
			}
		}
	}

	tradedOrders := make([]*Order, 0, len(s.orders))
	var reservedCredit, reservedIsu, tradedIsu, tradedCredit int64
	for _, o := range s.orders {
		if o.Removed() {
			continue
		}
		var order *Order
		for _, ro := range orders {
			if ro.ID == o.ID {
				order = &ro
				break
			}
		}
		if order == nil {
			// 自動的に消されたもの
			if o.Type == TradeTypeSell {
				return tradedOrders, errors.Errorf("GET /orders 売り注文が足りないか削除されています %d", o.ID)
			}
			ct := time.Now()
			o.ClosedAt = &ct
			continue
		}
		if order.Trade != nil && o.TradeID == 0 {
			tradedOrders = append(tradedOrders, order)
		}
		*o = *order
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
			reservedIsu += order.Amount
		case order.Type == TradeTypeBuy:
			// 買い注文
			reservedCredit += order.Amount * order.Price
		}
	}
	s.reservedIsu = reservedIsu
	s.reservedCredit = reservedCredit
	s.currentCredit = s.defaultCredit + tradedCredit
	s.currentIsu = s.defaultIsu + tradedIsu
	return tradedOrders, nil
}

func (s *NormalScenario) tryTrade(ctx context.Context) (ScoreType, error) {
	logicalCredit := s.currentCredit - s.reservedCredit
	logicalIsu := s.currentIsu - s.reservedIsu
	waiting := s.waitingOrders()
	if waiting >= rand.Intn(2)+4 { // 4,5になるので 5なら100%,4なら50%
		var o *Order
		var df int64
		for _, order := range s.orders {
			if order.ClosedAt == nil {
				var mdiff int64
				if order.Type == TradeTypeSell {
					mdiff = order.Price - s.highestBuyPrice
				} else {
					mdiff = s.lowestSellPrice - order.Price
				}
				if o == nil || df < mdiff {
					o = order
					df = mdiff
				}
			}
		}
		if err := s.c.DeleteOrders(ctx, o.ID); err != nil {
			if er, ok := err.(*ErrorWithStatus); ok && er.StatusCode == 404 {
				// 404エラーはありえるのでOK
				log.Printf("[INFO] delete 404 %s", er)
			} else {
				return ScoreTypeDeleteOrders, err
			}
		}
		now := time.Now()
		o.ClosedAt = &now
		return ScoreTypeDeleteOrders, nil
	}
	// 価格の決定

	// 成り行き行けるかどうか
	// 購入可能数
	buyable := logicalCredit / s.lowestSellPrice

	var (
		ot     string
		price  int64 = s.latestTradePrice
		amount int64 = rand.Int63n(s.unitIsu) + 1
	)
	// 価格は成り行き以外は前回価格からランダムに前後する
	switch rand.Intn(5) {
	case 1, 2:
		price++
	case 3, 4:
		price--
	}
	switch {
	case buyable/amount > 10:
		// 10回買い続けられるくらい資金が豊富
		// 成り行き買い注文
		ot = TradeTypeBuy
		price = s.lowestSellPrice
	case logicalIsu/amount > 10:
		// 10回売り続けられるくらい椅子が豊富
		// 成り行き売り注文
		ot = TradeTypeSell
		price = s.highestBuyPrice
	case logicalIsu < amount:
		// 売る椅子が無い = 買い確定
		ot = TradeTypeBuy
	case buyable < 1:
		// 買う金が無い = 売り確定
		ot = TradeTypeBuy
	case rand.Intn(2) == 0:
		ot = TradeTypeBuy
	default:
		ot = TradeTypeSell
	}

	if ot == TradeTypeBuy {
		if logicalCredit < price*amount {
			amount = logicalCredit / price
		}
	} else {
		if logicalIsu < amount {
			amount = logicalIsu
		}
	}

	if amount < 1 {
		return 0, nil
	}

	order, err := s.c.AddOrder(ctx, ot, amount, price)
	if err != nil {
		// 残高不足はOKとする
		if er, ok := err.(*ErrorWithStatus); ok && er.StatusCode == 400 && strings.Index(err.Error(), "残高") > -1 {
			log.Printf("[INFO] 残高不足 [user:%d, price:%d, amount:%d]", s.c.UserID(), price, amount)
			return ScoreTypePostOrders, nil
		}
		return ScoreTypePostOrders, err
	}
	s.orders = append(s.orders, order)

	return ScoreTypePostOrders, nil
}
