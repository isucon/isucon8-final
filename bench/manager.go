package bench

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

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
	investors []Investor
	scenarios []Scenario
	score     int64
	errors    []error
	logs      *bytes.Buffer

	nextLock     sync.Mutex
	investorLock sync.Mutex
	errorLock    sync.Mutex
	scenarioLock sync.Mutex
	level        uint
	totalivst    int
	overError    bool

	scounter int32
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
	logs := &bytes.Buffer{}
	return &Manager{
		logger:    NewLogger(io.MultiWriter(out, logs)),
		appep:     appep,
		bankep:    bankep,
		logep:     logep,
		rand:      rand,
		isubank:   bank,
		isulog:    isulog,
		idlist:    make(chan string, 10),
		investors: make([]Investor, 0, 5000),
		errors:    make([]error, 0, AllowErrorMax+10),
		logs:      logs,
	}, nil
}

func (c *Manager) Close() {
	for _, i := range c.investors {
		i.Close()
	}
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
		if i.IsRetired() {
			i.Close()
		} else {
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

func (c *Manager) AppendError(e error) error {
	if e == nil {
		return nil
	}
	c.errorLock.Lock()
	defer c.errorLock.Unlock()

	c.errors = append(c.errors, e)
	ec := len(c.errors)

	errorLimit := c.GetScore() / 20
	if errorLimit < AllowErrorMin {
		errorLimit = AllowErrorMin
	} else if errorLimit > AllowErrorMax {
		errorLimit = AllowErrorMax
	}
	if errorLimit <= int64(ec) {
		c.overError = true
		return errors.Errorf("エラー件数が規定を超過しました.")
	}
	return nil
}

func (c *Manager) ErrorCount() int {
	c.errorLock.Lock()
	defer c.errorLock.Unlock()
	return len(c.errors)
}

func (c *Manager) GetErrorsString() []string {
	r := make([]string, 0, len(c.errors))
	for _, e := range c.errors {
		r = append(r, e.Error())
	}
	return r
}

func (c *Manager) GetLogs() ([]string, error) {
	scan := bufio.NewScanner(c.logs)
	r := []string{}
	for scan.Scan() {
		r = append(r, scan.Text())
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

func (c *Manager) TotalScore() int64 {
	if c.overError {
		return 0
	}
	score := c.GetScore()
	demerit := score / (AllowErrorMax * 2)

	// エラーが多いと最大スコアが半分になる
	return score - demerit*int64(c.ErrorCount())
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

func (c *Manager) Initialize(ctx context.Context) error {
	c.nextLock.Lock()
	defer c.nextLock.Unlock()
	if err := c.isulog.Initialize(); err != nil {
		return errors.Wrap(err, "isuloggerの初期化に失敗しました。運営に連絡してください")
	}

	guest, err := NewClient(c.appep, "", "", "", InitTimeout, InitTimeout)
	if err != nil {
		return err
	}
	if err := guest.Initialize(ctx, c.bankep, c.isubank.AppID(), c.logep, c.isulog.AppID()); err != nil {
		return err
	}
	return nil
}

func (c *Manager) PreTest(ctx context.Context) error {
	t := &PreTester{
		appep:   c.appep,
		isubank: c.isubank,
		isulog:  c.isulog,
	}
	return t.Run(ctx)
}

func (c *Manager) PostTest(ctx context.Context) error {
	testInvestors := make([]testUser, 0, len(c.investors)+len(c.scenarios))
	for _, inv := range c.investors {
		if inv.IsSignin() && !inv.IsRetired() {
			if tu, ok := inv.(testUser); ok {
				testInvestors = append(testInvestors, tu)
			}
		}
	}
	for _, sc := range c.scenarios {
		if !sc.IsRetired() && sc.IsSignin() {
			if tu, ok := sc.(testUser); ok {
				testInvestors = append(testInvestors, tu)
			}
		}
	}
	t := &PostTester{
		appep:   c.appep,
		isubank: c.isubank,
		isulog:  c.isulog,
		users:   testInvestors,
	}
	return t.Run(ctx)
}

func (c *Manager) Start() ([]taskworker.Task, error) {
	c.nextLock.Lock()
	defer c.nextLock.Unlock()

	basePrice := 5105

	tasks := make([]taskworker.Task, 0, DefaultWorkers+BruteForceWorkers)
	for i := 0; i < DefaultWorkers; i++ {
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
	accounts := []string{"5gf4syuu", "qgar5ge8dv4g", "gv3bsxzejbb4", "jybp5gysw279"}
	for i := 0; i < BruteForceWorkers; i++ {
		cl, err := NewClient(c.appep, accounts[i], "わからない", "12345", ClientTimeout, RetireTimeout)
		if err != nil {
			return nil, err
		}
		investor := NewBruteForceInvestor(cl)
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
	addInvestors := func(num int, unitamount, price int64) error {
		for i := 0; i < num; i++ {
			cl, err := c.newClient()
			if err != nil {
				return err
			}
			var investor Investor
			if i%2 == 1 {
				investor = NewRandomInvestor(cl, price*1000, 0, unitamount, price-2)
			} else {
				investor = NewRandomInvestor(cl, 0, unitamount*100, unitamount, price+5)
			}
			tasks = append(tasks, taskworker.NewExecTask(func(_ context.Context) error {
				if investor.Credit() > 0 {
					c.isubank.AddCredit(investor.BankID(), investor.Credit())
				}
				c.AddInvestor(investor)
				return nil
			}, 0))
		}
		return nil
	}
	start := 2 // 一度に投入する数
	for _, investor := range c.investors {
		if !investor.IsStarted() {
			tasks = append(tasks, investor.Start())
			start--
		}
		if start <= 0 {
			break
		}
	}

	var latestTradePrice int64 = 5000
	var addByShare int
	for _, investor := range c.investors {
		if !investor.IsStartCompleted() {
			continue
		}
		if investor.IsRetired() {
			continue
		}
		if task := investor.Next(); task != nil {
			tasks = append(tasks, task)
		}
		for _, trade := range investor.SharedTrades() {
			if err := addInvestors(AddUsersOnShare, trade.Amount, trade.Price); err != nil {
				return nil, err
			}
			addByShare++
		}
		latestTradePrice = investor.LatestTradePrice()
	}
	if addByShare > 0 {
		c.Logger().Printf("SNSでシェアされたためアクティブユーザーが増加しました[%d]", addByShare)
	}

	score := c.GetScore()
	// 自然増加
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
		c.Logger().Printf("アクティブユーザーが自然増加します")

		if err := addInvestors(AddUsersOnNatural, int64(c.level+1), latestTradePrice); err != nil {
			return nil, err
		}
	}
	return tasks, nil
}

func (c *Manager) NewScenario() (Scenario, error) {
	var credit, isu, unit int64
	n := atomic.AddInt32(&c.scounter, 1)
	switch {
	case n%9 == 3 && n < 35: // 3, 12, 21, 30
		accounts := []string{"5gf4syuu", "qgar5ge8dv4g", "gv3bsxzejbb4", "jybp5gysw279"}
		cl, err := NewClient(c.appep, accounts[int(n/9)], "わからない", "12345", ClientTimeout, RetireTimeout)
		if err != nil {
			return nil, err
		}
		return NewBruteForceScenario(cl), nil
	case n < 16:
		credit, isu, unit = 30000, 5, 1
	case n == 20:
		// 成り行き買い
		credit, isu, unit = 500000, 0, 5
	case n == 21:
		// 成り行き売り
		credit, isu, unit = 0, 100, 5
	default:
		credit, isu, unit = 35000, 7, 3
	}
	cl, err := c.newClient()
	if err != nil {
		return nil, err
	}
	return NewNormalScenario(cl, credit, isu, unit), nil
}

func (c *Manager) ScenarioStart(ctx context.Context) error {
	smchan := make(chan ScoreMsg, 2000)
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var err error

	go func() {
		defer cancel()
		if err = c.recvScoreMsg(cctx, smchan); err != nil {
			c.Logger().Printf("ベンチマークを終了します: %s", err)
		}
	}()

	go c.tickScenario(cctx, smchan)

	if err := c.startScenarios(cctx, smchan, DefaultWorkers); err != nil {
		return nil
	}
	<-cctx.Done()
	handleContextErr(cctx.Err())
	return err
}

func (c *Manager) startScenarios(ctx context.Context, smchan chan ScoreMsg, num int) error {
	for i := 0; i < num; i++ {
		scenario, err := c.NewScenario()
		if err != nil {
			return err
		}
		go func() {
			if scenario.Credit() > 0 {
				c.isubank.AddCredit(scenario.BankID(), scenario.Credit())
			}
			c.scenarioLock.Lock()
			// add
			if err := scenario.Start(ctx, smchan); err != nil {
				log.Printf("[INFO] scenario.Start failed. %s", err)
			}
			c.scenarios = append(c.scenarios, scenario)
			c.scenarioLock.Unlock()
		}()
	}
	return nil
}

func (c *Manager) tickScenario(ctx context.Context, smchan chan ScoreMsg) {
	for {
		select {
		case <-ctx.Done():
			handleContextErr(ctx.Err())
			return
		case <-time.After(TickerInterval):
			score := c.GetScore()
			// 自然増加
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
				c.Logger().Printf("アクティブユーザーが自然増加します")
				if e := c.startScenarios(ctx, smchan, AddUsersOnNatural); e != nil {
					log.Printf("[INFO] scenario.Start failed. %s", e)
				}
			}
		}
	}
}

func (c *Manager) recvScoreMsg(ctx context.Context, smchan chan ScoreMsg) error {
	for {
		select {
		case <-ctx.Done():
			handleContextErr(ctx.Err())
			return nil
		case s := <-smchan:
			if s.err != nil {
				if s.err == ErrAlreadyRetired {
					continue
				}
				c.Logger().Printf("error: %s", s.err)
				if e := c.AppendError(s.err); e != nil {
					return e
				}
			} else {
				c.AddScore(s.st.Score())
				scoreboard.Add(s.st, 1)
				if s.sns {
					if e := c.startScenarios(ctx, smchan, AddUsersOnShare); e != nil {
						log.Printf("[INFO] scenario.Start failed. %s", e)
					} else {
						c.Logger().Printf("SNSでシェアされたためアクティブユーザーが増加しました")
					}
				}
			}
		}
	}
}
