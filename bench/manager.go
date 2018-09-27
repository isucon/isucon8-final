package bench

import (
	"context"
	"io"
	"log"
	"sync"
	"sync/atomic"

	"github.com/ken39arg/isucon2018-final/bench/isubank"
	"github.com/ken39arg/isucon2018-final/bench/isulog"
	"github.com/ken39arg/isucon2018-final/bench/taskworker"
	"github.com/pkg/errors"
)

type Manager struct {
	logger    *log.Logger
	appep     string
	bankep    string
	logep     string
	rand      *Random
	isubank   *isubank.Isubank
	isulog    *isulog.Isulog
	idlist    chan string
	closed    chan struct{}
	investors []Investor
	score     int64
	errcount  int64

	nextLock     sync.Mutex
	investorLock sync.Mutex
	level        uint
	totalivst    int
}

func NewManager(out io.Writer, appep, bankep, logep, internalbank, internallog string) (*Manager, error) {
	rand, err := NewRandom()
	if err != nil {
		return nil, err
	}
	bank, err := isubank.NewIsubank(internalbank, rand.ID())
	if err != nil {
		return nil, err
	}
	isulog, err := isulog.NewIsulog(internallog, rand.ID())
	if err != nil {
		return nil, err
	}
	return &Manager{
		logger:    NewLogger(out),
		appep:     appep,
		bankep:    bankep,
		logep:     logep,
		rand:      rand,
		isubank:   bank,
		isulog:    isulog,
		idlist:    make(chan string, 10),
		closed:    make(chan struct{}),
		investors: make([]Investor, 0, 5000),
	}, nil
}

// benchに影響を与えないようにidは予め用意しておく
func (c *Manager) RunIDFetcher(ctx context.Context) {
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

func (c *Manager) FetchNewID() string {
	return <-c.idlist
}

func (c *Manager) AddInvestor(i Investor) {
	c.investorLock.Lock()
	defer c.investorLock.Unlock()
	c.investors = append(c.investors, i)
	c.totalivst++
}

func (c *Manager) PurgeInvestor() {
	c.investorLock.Lock()
	defer c.investorLock.Unlock()
	cleared := make([]Investor, 0, cap(c.investors))
	for _, i := range c.investors {
		if !i.IsRetired() {
			cleared = append(cleared, i)
		}
	}
	c.investors = cleared
}

func (c *Manager) AddScore(score int64) {
	atomic.AddInt64(&c.score, score)
}

func (c *Manager) GetScore() int64 {
	return atomic.LoadInt64(&c.score)
}

func (c *Manager) IncrErr() error {
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

func (c *Manager) ErrorCount() int64 {
	return atomic.LoadInt64(&c.errcount)
}

func (c *Manager) TotalScore() int64 {
	score := c.GetScore()
	demerit := score / (AllowErrorMax * 2)

	// エラーが多いと最大スコアが半分になる
	return score - demerit*c.ErrorCount()
}

func (c *Manager) GetLevel() uint {
	return c.level
}

func (c *Manager) AllInvestors() int {
	return c.totalivst
}

func (c *Manager) ActiveInvestors() int {
	return len(c.investors)
}

func (c *Manager) FindInvestor(bankID string) Investor {
	for _, i := range c.investors {
		if i.BankID() == bankID {
			return i
		}
	}
	return nil
}

func (c *Manager) newClient() (*Client, error) {
	return NewClient(c.appep, c.FetchNewID(), c.rand.Name(), c.rand.Password(), ClientTimeout, RetireTimeout)
}

func (c *Manager) Logger() *log.Logger {
	return c.logger
}

func (c *Manager) Initialize() error {
	c.nextLock.Lock()
	defer c.nextLock.Unlock()
	if err := c.isulog.Initialize(); err != nil {
		return errors.Wrap(err, "isuloggerの初期化に失敗しました。運営に連絡してください")
	}

	guest, err := NewClient(c.appep, "", "", "", InitTimeout, InitTimeout)
	if err != nil {
		return err
	}
	if err := guest.Initialize(c.bankep, c.isubank.AppID(), c.logep, c.isulog.AppID()); err != nil {
		return err
	}
	return nil
}

func (c *Manager) PreTest() error {
	t := &PreTester{
		appep:   c.appep,
		isubank: c.isubank,
		isulog:  c.isulog,
	}
	return t.Run()
}

func (c *Manager) PostTest() error {
	t := &PostTester{
		appep:     c.appep,
		isubank:   c.isubank,
		isulog:    c.isulog,
		investors: c.investors,
	}
	return t.Run()
}

func (c *Manager) Start() ([]taskworker.Task, error) {
	c.nextLock.Lock()
	defer c.nextLock.Unlock()

	basePrice := 5105

	tasks := make([]taskworker.Task, 0, AddWorkersByLevel)
	for i := 0; i < AddWorkersByLevel; i++ {
		cl, err := c.newClient()
		if err != nil {
			return nil, err
		}
		var investor Investor
		if i%2 == 1 {
			investor = NewRandomInvestor(cl, 100000, 0, 1, int64(basePrice+i/2))
		} else {
			investor = NewRandomInvestor(cl, 0, 5, 1, int64(basePrice+i/2))
		}
		if investor.Credit() > 0 {
			c.isubank.AddCredit(investor.BankID(), investor.Credit())
		}
		c.AddInvestor(investor)
		tasks = append(tasks, investor.Start())
	}
	return tasks, nil
}

func (c *Manager) Next() ([]taskworker.Task, error) {
	c.nextLock.Lock()
	defer c.nextLock.Unlock()

	c.PurgeInvestor()

	if c.ActiveInvestors() == 0 {
		return nil, errors.New("アクティブユーザーがいなくなりました")
	}

	tasks := []taskworker.Task{}
	for _, investor := range c.investors {
		// 初期以外はnextのタイミングで一人づつ投入
		if !investor.IsStarted() {
			tasks = append(tasks, investor.Start())
			break
		}
	}

	for _, investor := range c.investors {
		if !investor.IsSignin() {
			continue
		}
		if investor.IsRetired() {
			continue
		}
		if task := investor.Next(); task != nil {
			tasks = append(tasks, task)
		}
	}

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
		latestTradePrice := c.investors[0].LatestTradePrice()
		if latestTradePrice == 0 {
			latestTradePrice = 100
		}
		c.level++
		c.Logger().Printf("アクティブユーザーが自然増加します")

		// 2人追加
		unitamount := int64(c.level + 1)
		for i := 0; i < 2; i++ {
			cl, err := c.newClient()
			if err != nil {
				return nil, err
			}
			var investor Investor
			if i%2 == 1 {
				investor = NewRandomInvestor(cl, latestTradePrice*1000, 0, unitamount, latestTradePrice-2)
			} else {
				investor = NewRandomInvestor(cl, 0, unitamount*100, unitamount, latestTradePrice+5)
			}
			tasks = append(tasks, taskworker.NewExecTask(func(_ context.Context) error {
				if investor.Credit() > 0 {
					c.isubank.AddCredit(investor.BankID(), investor.Credit())
				}
				c.AddInvestor(investor)
				return nil
			}, 0))
		}
	}
	return tasks, nil
}
