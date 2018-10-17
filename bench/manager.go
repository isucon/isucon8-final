package bench

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ken39arg/isucon2018-final/bench/isubank"
	"github.com/ken39arg/isucon2018-final/bench/isulog"
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
	scenarios []Scenario
	score     int64
	errors    []error
	logs      *bytes.Buffer

	errorLock    sync.Mutex
	scenarioLock sync.Mutex
	level        uint
	overError    bool

	scounter   int32
	scoreboard *ScoreBoard
	tupointer  int
	testusers  []TestUser
}

func NewManager(out io.Writer, appep, bankep, logep, internalbank, internallog string) (*Manager, error) {
	rnd, err := NewRandom()
	if err != nil {
		return nil, err
	}
	bank, err := isubank.NewIsubank(internalbank, rnd.ID())
	if err != nil {
		return nil, err
	}
	isulog, err := isulog.NewIsulog(internallog, rnd.ID())
	if err != nil {
		return nil, err
	}
	scoreboard := &ScoreBoard{
		count: make(map[ScoreType]int64, 20),
	}
	_testusers := make([]TestUser, len(testUsers))
	copy(_testusers, testUsers)
	for i := range _testusers {
		j := rand.Intn(i + 1)
		_testusers[i], _testusers[j] = _testusers[j], _testusers[i]
	}
	logs := &bytes.Buffer{}
	return &Manager{
		logger:     NewLogger(io.MultiWriter(out, logs)),
		appep:      appep,
		bankep:     bankep,
		logep:      logep,
		rand:       rnd,
		isubank:    bank,
		isulog:     isulog,
		idlist:     make(chan string, 10),
		errors:     make([]error, 0, AllowErrorMax+10),
		logs:       logs,
		scenarios:  make([]Scenario, 0, 2000),
		scoreboard: scoreboard,
		testusers:  _testusers,
	}, nil
}

func (c *Manager) Close() {
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

func (c *Manager) AllUsers() int {
	return len(c.scenarios)
}

func (c *Manager) ActiveUsers() int {
	n := 0
	for _, sc := range c.scenarios {
		if !sc.IsRetired() {
			n++
		}
	}
	return n
}

func (c *Manager) Logger() *log.Logger {
	return c.logger
}

func (c *Manager) Initialize(ctx context.Context) error {
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
	testUsers := make([]testUser, 0, len(c.scenarios))
	for _, sc := range c.scenarios {
		if !sc.IsRetired() && sc.IsSignin() {
			if tu, ok := sc.(testUser); ok {
				testUsers = append(testUsers, tu)
			}
		}
	}
	t := &PostTester{
		appep:   c.appep,
		isubank: c.isubank,
		isulog:  c.isulog,
		users:   testUsers,
	}
	return t.Run(ctx)
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

func (c *Manager) nextTestUser() TestUser {
	i := c.tupointer
	c.tupointer++
	if c.tupointer >= len(c.testusers) {
		c.tupointer = 0
	}
	return c.testusers[i]
}

func (c *Manager) newScenario() (Scenario, error) {
	var credit, isu, unit int64
	var justprice bool
	n := atomic.AddInt32(&c.scounter, 1)
	switch {
	case n%10 == 3:
		for {
			tu := c.nextTestUser()
			if tu.Cost == 10 {
				cl, err := NewClient(c.appep, tu.BankID, tu.Name, "12345", ClientTimeout, RetireTimeout)
				if err != nil {
					return nil, err
				}
				log.Printf("[DEBUG] add BruteForce %s", tu.BankID)
				return NewBruteForceScenario(cl), nil
			}
		}
	case n%5 == 2:
		tu := c.nextTestUser()
		cl, err := NewClient(c.appep, tu.BankID, tu.Name, tu.Pass, ClientTimeout, RetireTimeout)
		if err != nil {
			return nil, err
		}
		// この人達を成り行きにはしたくないけどしょうがない
		credit, err = c.isubank.GetCredit(tu.BankID)
		if err != nil {
			return nil, err
		}
		log.Printf("[DEBUG] add exists user")
		return NewExistsUserScenario(cl, credit, 10, 3, false), nil
	case n == 10 || n == 20 || n == 30:
		// 成り行き買い
		credit, isu, unit, justprice = 5000000, 0, 5, true
	case n == 11 || n == 21 || n == 31:
		// 成り行き売り
		credit, isu, unit, justprice = 0, 200, 5, true
	case n < 16:
		credit, isu, unit = 30000, 5, 1
	default:
		credit, isu, unit = 35000, 7, 3
	}
	cl, err := NewClient(c.appep, c.FetchNewID(), c.rand.Name(), c.rand.Password(), ClientTimeout, RetireTimeout)
	if err != nil {
		return nil, err
	}
	if credit > 0 {
		c.isubank.AddCredit(cl.bankid, credit)
	}
	return NewNormalScenario(cl, credit, isu, unit, justprice), nil
}

func (c *Manager) startScenarios(ctx context.Context, smchan chan ScoreMsg, num int) error {
	for i := 0; i < num; i++ {
		go func() {
			time.Sleep(time.Duration(rand.Int63n(100)) * time.Millisecond)
			scenario, err := c.newScenario()
			if err != nil {
				log.Printf("[WARN] newScenario failed. err: %s", err)
				return
			}
			// add
			if err := scenario.Start(ctx, smchan); err != nil {
				switch errors.Cause(err) {
				case context.DeadlineExceeded, context.Canceled:
				default:
					log.Printf("[INFO] scenario.Start user:%s, failed. %s", scenario.BankID(), err)
				}
			} else {
				c.scenarioLock.Lock()
				c.scenarios = append(c.scenarios, scenario)
				c.scenarioLock.Unlock()
			}
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
				switch errors.Cause(s.err) {
				case ErrAlreadyRetired, context.DeadlineExceeded, context.Canceled:
				default:
					c.Logger().Printf("error: %s", s.err)
					if e := c.AppendError(s.err); e != nil {
						return e
					}
				}
			} else {
				c.AddScore(s.st.Score())
				c.scoreboard.Add(s.st)
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
