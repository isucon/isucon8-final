package bench

import (
	"context"
	"io"
	"log"
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
	idlist    chan string
	closed    chan struct{}
	investors []Investor
}

func NewContext(out io.Writer, appep, bankep, logep string) (*Context, error) {
	rand, err := NewRandom()
	if err != nil {
		return nil, err
	}
	isubank, err := NewIsubank(bankep)
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
	c.investors = append(c.investors, i)
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

func (c *Context) Initialize() error {
	client, err := NewClient(c.appep, "", "", "", InitTimeout)
	if err != nil {
		return err
	}
	return client.Initialize(c.bankep, c.bankappid, c.logep, c.logappid)
}

func (c *Context) Logger() *log.Logger {
	return c.logger
}
