package bench

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type Scenario interface {
	Start(context.Context, chan ScoreMsg) error
	IsSignin() bool
	IsRetired() bool
	BankID() string
	Credit() int64
}

type baseScenario struct {
	c *Client
}

func (s *baseScenario) IsSignin() bool {
	return 0 < s.c.UserID()
}

func (s *baseScenario) IsRetired() bool {
	return s.c.IsRetired()
}

func (s *baseScenario) UserID() int64 {
	return s.c.UserID()
}

func (s *baseScenario) BankID() string {
	return s.c.bankid
}

func (s *baseScenario) Credit() int64 {
	return 0
}

func (s *baseScenario) Client() *Client {
	return s.c
}

type normalScenario struct {
	*baseScenario

	lowestSellPrice  int64
	highestBuyPrice  int64
	latestTradePrice int64
	enableShare      bool
	orders           []*Order
	ordersLock       sync.Mutex

	unitIsu        int64
	defaultIsu     int64
	defaultCredit  int64
	reservedIsu    int64
	reservedCredit int64
	currentIsu     int64
	currentCredit  int64

	actionchan chan struct{}
	existed    bool
	ignoretest bool
	justprice  bool
}

func newNormalScenario(c *Client, credit, isu, unit int64, justprice bool) *normalScenario {
	return &normalScenario{
		baseScenario:  &baseScenario{c},
		defaultCredit: credit,
		defaultIsu:    isu,
		currentCredit: credit,
		currentIsu:    isu,
		unitIsu:       unit,
		orders:        make([]*Order, 0, 60),
		actionchan:    make(chan struct{}, BenchMarkTime/PollingInterval),
		justprice:     justprice,
	}
}

func NewNormalScenario(c *Client, credit, isu, unit int64, justprice bool) Scenario {
	return newNormalScenario(c, credit, isu, unit, justprice)
}

func NewExistsUserScenario(c *Client, credit, isu, unit int64, justprice bool) Scenario {
	s := newNormalScenario(c, credit, isu, unit, justprice)
	s.existed = true
	s.ignoretest = true
	return s
}

func (s *normalScenario) Orders() []*Order {
	return s.orders
}

func (s *normalScenario) Credit() int64 {
	return s.currentCredit
}

func (s *normalScenario) Ignore() bool {
	return s.ignoretest
}

func (s *normalScenario) FetchOrders(ctx context.Context) error {
	_, err := s.fetchOrders(ctx, true)
	return err
}

func (s *normalScenario) waitingOrders() int {
	c := 0
	for _, o := range s.orders {
		if o.ClosedAt == nil {
			c++
		}
	}
	return c
}

func (s *normalScenario) Start(ctx context.Context, smchan chan ScoreMsg) error {
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

	if !s.existed {
		err = s.c.Signup(ctx)
		smchan <- ScoreMsg{st: ScoreTypeSignup, err: err}
		if err != nil {
			return errors.Wrap(err, "アカウントを作成できませんでした")
		}
	}

	err = s.c.Signin(ctx)
	smchan <- ScoreMsg{st: ScoreTypeSignin, err: err}
	if err != nil {
		return errors.Wrap(err, "ログインできませんでした")
	}

	_, err = s.fetchOrders(ctx, false)
	smchan <- ScoreMsg{st: ScoreTypeGetOrders, err: err}
	if err != nil {
		return errors.Wrap(err, "注文履歴の取得に失敗しました")
	}

	go s.runAction(ctx, smchan)

	go s.runInfoLoop(ctx, smchan)

	return nil
}

func (s *normalScenario) runInfoLoop(ctx context.Context, smchan chan ScoreMsg) {
	var cursor int64
	for {
		select {
		case <-ctx.Done():
			handleContextErr(ctx.Err())
			return
		default:
			if s.c.IsRetired() {
				return
			}
			nextLoopUnlock := time.After(PollingInterval)
			next, traded, err := s.fetchInfo(ctx, cursor)
			smchan <- ScoreMsg{st: ScoreTypeGetInfo, err: err}
			if err != nil {
				if _, ok := err.(*ErrElapsedTimeOverRetire); ok {
					return
				}
			}
			if next > 0 {
				cursor = next
			}
			if traded {
				go func() {
					if s.c.IsRetired() {
						return
					}
					tradedOrders, err := s.fetchOrders(ctx, false)
					smchan <- ScoreMsg{st: ScoreTypeGetOrders, err: err}
					if err == nil {
						for range tradedOrders {
							smchan <- ScoreMsg{st: ScoreTypeTradeSuccess, sns: s.enableShare}
						}
					} else {
						if _, ok := err.(*ErrElapsedTimeOverRetire); ok {
							return
						}
					}
				}()
			}
			s.actionchan <- struct{}{}
			<-nextLoopUnlock
		}
	}
}

func (s *normalScenario) runAction(ctx context.Context, smchan chan ScoreMsg) {
	var gapCount int64
	for {
		select {
		case <-ctx.Done():
			handleContextErr(ctx.Err())
			return
		case <-s.actionchan:
			if s.c.IsRetired() {
				return
			}
			nextActionLock := time.After(OrderUpdateInterval)
			st, err := s.tryTrade(ctx)
			if st == 0 {
				continue
			}
			smchan <- ScoreMsg{st: st, err: err}
			if err != nil {
				if _, ok := err.(*ErrElapsedTimeOverRetire); ok {
					return
				}
				continue
			}
			tradedOrders, err := s.fetchOrders(ctx, false)
			smchan <- ScoreMsg{st: ScoreTypeGetOrders, err: err}
			if err == nil {
				for range tradedOrders {
					smchan <- ScoreMsg{st: ScoreTypeTradeSuccess, sns: s.enableShare}
				}
			} else {
				if _, ok := err.(*ErrElapsedTimeOverRetire); ok {
					return
				}
			}
			<-nextActionLock
			// 取引可能状態が続くとtradeが渋滞しているはずなのでインターバルを伸ばす
			if s.lowestSellPrice < s.highestBuyPrice {
				gapCount++
				if gapCount >= 5 {
					time.Sleep(time.Duration((gapCount-5)*100) * time.Millisecond)
				}
			} else {
				gapCount = 0
			}
		}
	}
}

func (s *normalScenario) fetchInfo(ctx context.Context, cursor int64) (int64, bool, error) {
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

func (s *normalScenario) fetchOrders(ctx context.Context, skipReflectCheck bool) ([]*Order, error) {
	s.ordersLock.Lock()
	defer s.ordersLock.Unlock()
	orders, err := s.c.GetOrders(ctx)
	if err != nil {
		return nil, err
	}
	if len(s.orders) > 0 && !skipReflectCheck {
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
	for _, o := range s.orders {
		var order *Order
		for _, ro := range orders {
			if ro.ID == o.ID {
				order = &ro
				break
			}
		}
		if order == nil {
			if !o.Removed() {
				// 自動的に消されたもの
				if o.Type == TradeTypeSell {
					return tradedOrders, errors.Errorf("GET /orders 売り注文が足りないか削除されています %d", o.ID)
				}
				ct := time.Now()
				o.ClosedAt = &ct
			}
			continue
		}
		if order.Trade != nil && o.TradeID == 0 {
			tradedOrders = append(tradedOrders, order)
		}
		*o = *order
	}

	var reservedCredit, reservedIsu, tradedIsu, tradedCredit int64
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

func (s *normalScenario) tryTrade(ctx context.Context) (ScoreType, error) {
	s.ordersLock.Lock()
	defer s.ordersLock.Unlock()
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
	var (
		ot      string
		price   int64 = s.latestTradePrice
		amount  int64 = rand.Int63n(s.unitIsu) + 1
		buyable int64
	)
	if s.lowestSellPrice > 0 {
		buyable = logicalCredit / s.lowestSellPrice
	} else {
		buyable = logicalCredit / s.latestTradePrice
	}
	// 価格は成り行き以外は前回価格からランダムに前後する
	switch rand.Intn(5) {
	case 1, 2:
		price++
	case 3, 4:
		price--
	}
	switch {
	case buyable/amount > 10 && s.justprice:
		// 10回買い続けられるくらい資金が豊富
		// 成り行き買い注文
		ot = TradeTypeBuy
		price = s.lowestSellPrice
	case logicalIsu/amount > 10 && s.justprice:
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

type bruteForceScenario struct {
	*baseScenario
	defpass string
}

func NewBruteForceScenario(c *Client) Scenario {
	return &bruteForceScenario{
		baseScenario: &baseScenario{c},
		defpass:      c.pass,
	}
}

func (s *bruteForceScenario) Start(ctx context.Context, smchan chan ScoreMsg) error {
	var cursor int64
	go func() {
		n := 0
		b := 0
		for {
			select {
			case <-ctx.Done():
				handleContextErr(ctx.Err())
				return
			default:
				if s.c.IsRetired() {
					return
				}
				actionInterval := time.After(BruteForceDelay)
				err := s.c.Top(ctx)
				smchan <- ScoreMsg{st: ScoreTypeGetTop, err: err}
				if err != nil {
					if _, ok := err.(*ErrElapsedTimeOverRetire); ok {
						return
					}
					<-actionInterval
					continue
				}
				info, err := s.c.Info(ctx, cursor)
				smchan <- ScoreMsg{st: ScoreTypeGetInfo, err: err}
				if err != nil {
					if _, ok := err.(*ErrElapsedTimeOverRetire); ok {
						return
					}
					<-actionInterval
					continue
				}
				cursor = info.Cursor

				if b > 0 {
					b--
					//log.Printf("[DEBUG] skip signin by 403")
					smchan <- ScoreMsg{st: ScoreTypeSignin}
					<-actionInterval
					continue
				}

				s.c.pass = fmt.Sprintf("password%03d", rand.Intn(1000))
				n++
				err = s.c.Signin(ctx)
				if err == nil {
					err = errors.Errorf("不正ログインに成功しました")
					n = 0
				} else if e, ok := err.(*ErrorWithStatus); ok {
					switch e.StatusCode {
					case 403:
						if n > 5 {
							err = nil
							b = n
						}
					case 404:
						err = nil
					default:
						n = 0
					}
				}
				smchan <- ScoreMsg{st: ScoreTypeSignin, err: err}
				if err != nil {
					if _, ok := err.(*ErrElapsedTimeOverRetire); ok {
						return
					}
				}
				<-actionInterval
			}
		}
	}()

	return nil
}

func handleContextErr(err error) {
	switch err {
	case context.DeadlineExceeded, context.Canceled, nil:
	default:
		log.Printf("[WARN] context error %s", err)
	}
}
