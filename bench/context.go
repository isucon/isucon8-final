package bench

import (
	"context"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

type Context struct {
	logger    *log.Logger
	appep     string
	bankep    string
	logep     string
	bankappid string
	logappid  string
	rand      *Random
	isubank   *Isubank
	guest     *Client
	idlist    chan string
	closed    chan struct{}
	investors []Investor
	score     int64
	errcount  int64

	lastTrade    Trade
	nextLock     sync.Mutex
	investorLock sync.Mutex
	level        uint

	lastTradePorring time.Time
}

func NewContext(out io.Writer, appep, bankep, logep, internalbank string) (*Context, error) {
	rand, err := NewRandom()
	if err != nil {
		return nil, err
	}
	isubank, err := NewIsubank(internalbank)
	if err != nil {
		return nil, err
	}
	guest, err := NewClient(appep, "", "", "", InitTimeout)
	if err != nil {
		return nil, err
	}
	return &Context{
		logger:    NewLogger(out),
		appep:     appep,
		bankep:    bankep,
		logep:     logep,
		bankappid: rand.ID(),
		logappid:  rand.ID(),
		rand:      rand,
		isubank:   isubank,
		guest:     guest,
		idlist:    make(chan string, 10),
		closed:    make(chan struct{}),
		investors: make([]Investor, 0, 5000),
	}, nil
}

// benchに影響を与えないようにidは予め用意しておく
func (c *Context) RunIDFetcher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			id := c.rand.ID()
			if err := c.isubank.NewBankID(id); err != nil {
				log.Printf("new bankid failed. %s", err)
			}
			c.idlist <- id
		}
	}
}

func (c *Context) FetchNewID() string {
	return <-c.idlist
}

func (c *Context) AddInvestor(i Investor) {
	c.investorLock.Lock()
	defer c.investorLock.Unlock()
	c.investors = append(c.investors, i)
}

func (c *Context) RemoveInvestor(i Investor) {
	c.investorLock.Lock()
	defer c.investorLock.Unlock()
	cleared := make([]Investor, 0, cap(c.investors))
	for _, ii := range c.investors {
		if i.BankID() != ii.BankID() {
			cleared = append(cleared, ii)
		}
	}
	c.investors = cleared
}

func (c *Context) AddScore(score int64) {
	atomic.AddInt64(&c.score, score)
}

func (c *Context) GetScore() int64 {
	return atomic.LoadInt64(&c.score)
}

func (c *Context) IncrErr() error {
	ec := atomic.AddInt64(&c.errcount, 1)

	errorLimit := c.GetScore() / 20
	if errorLimit < AllowErrorMin {
		errorLimit = AllowErrorMin
	} else if errorLimit > AllowErrorMax {
		errorLimit = AllowErrorMax
	}
	if errorLimit <= ec {
		return errors.Errorf("エラー件数が規定を超過しました.")
	}
	return nil
}

func (c *Context) ErrorCount() int64 {
	return atomic.LoadInt64(&c.errcount)
}

func (c *Context) TotalScore() int64 {
	score := c.GetScore()
	demerit := score / (AllowErrorMax * 2)

	// エラーが多いと最大スコアが半分になる
	return score - demerit*c.ErrorCount()
}

func (c *Context) FindInvestor(bankID string) Investor {
	for _, i := range c.investors {
		if i.BankID() == bankID {
			return i
		}
	}
	return nil
}

func (c *Context) NewClient() (*Client, error) {
	return NewClient(c.appep, c.FetchNewID(), c.rand.Name(), c.rand.Password(), ClientTimeout)
}

func (c *Context) Logger() *log.Logger {
	return c.logger
}

func (c *Context) Start() ([]Task, error) {
	c.nextLock.Lock()
	defer c.nextLock.Unlock()

	if err := c.guest.Initialize(c.bankep, c.bankappid, c.logep, c.logappid); err != nil {
		return nil, err
	}
	firstInvestor := len(firstprams)
	tasks := make([]Task, 0, firstInvestor)
	for _, p := range firstprams {
		cl, err := c.NewClient()
		if err != nil {
			return nil, err
		}
		investor := NewRandomInvestor(cl, p.credit, p.isu, p.unitamount, p.unitprice)
		c.isubank.AddCredit(investor.BankID(), investor.Credit())
		c.AddInvestor(investor)
		tasks = append(tasks, investor.Start())
	}
	return tasks, nil
}

func (c *Context) Next() ([]Task, error) {
	c.nextLock.Lock()
	defer c.nextLock.Unlock()

	var trades []Trade
	var err error
	if c.lastTradePorring.IsZero() || time.Now().Sub(c.lastTradePorring) >= TradePollingInterval {
		trades, err = c.guest.Trades()
		if err != nil {
			return nil, errors.Wrap(err, "GET /trades の取得に失敗しました")
		}
		c.AddScore(GetTradesScore)
	}

	tasks := []Task{}
	for _, investor := range c.investors {
		// 初期以外はnextのタイミングで一人づつ投入
		if !investor.IsSignin() {
			tasks = append(tasks, investor.Start())
			break
		}
	}

	if trades != nil && len(trades) > 0 && trades[0].ID != c.lastTrade.ID {
		for _, trade := range trades {
			if trade.ID > c.lastTrade.ID {
				// これは個別のほうが良いか
				c.AddScore(TradeSuccessScore)
			} else {
				break
			}
		}
		for _, investor := range c.investors {
			if !investor.IsSignin() {
				continue
			}
			if task := investor.Next(trades); task != nil {
				tasks = append(tasks, task)
			}
		}
		c.lastTrade = trades[0]
		score := c.GetScore()
		for {
			// levelup
			nextScore := (1 << c.level) * 100
			if score < int64(nextScore) {
				break
			}
			if AllowErrorMin < c.ErrorCount() {
				// エラー回数がscoreの5%以上あったらワーカーレベルは上がらない
				break
			}
			c.level++
			c.Logger().Printf("ワーカーレベルが上がります")

			// 10人追加
			unitamount := int64(c.level * 5)
			for i := 0; i < 10; i++ {
				cl, err := c.NewClient()
				if err != nil {
					return nil, err
				}
				var investor Investor
				if i%2 == 1 {
					investor = NewRandomInvestor(cl, c.lastTrade.Price*1000, 0, unitamount, c.lastTrade.Price-1)
				} else {
					investor = NewRandomInvestor(cl, 1, unitamount*100, unitamount, c.lastTrade.Price+1)
				}
				tasks = append(tasks, NewExecTask(func(_ context.Context) error {
					c.isubank.AddCredit(investor.BankID(), investor.Credit())
					c.AddInvestor(investor)
					return nil
				}, 0))
			}
		}
	}
	return tasks, nil
}

// まずは100円で成立させる
var firstprams = []struct {
	credit, isu, unitamount, unitprice int64
}{
	{100000, 0, 3, 100},
	{1, 1000, 3, 100},
	{100000, 0, 3, 100},
	{1, 1000, 3, 100},
	{1000, 0, 1, 100},
	{1, 10, 1, 100},
	{1000, 0, 1, 100},
	{1, 10, 1, 100},
	{1000, 0, 1, 100},
	{1, 10, 1, 100},
}
