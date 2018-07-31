package bench

import (
	"context"
	"time"
)

type Runner struct {
	bctx    *Context
	timeout time.Duration
}

func NewRunner(bctx *Context, timeout time.Duration) {
	return &Runner{
		bctx:    bctx,
		timeout: timeout,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	worker := NewWorker()

	c := r.bctx

	cctx, ccancel := context.WithCancel(ctx)
	defer ccancel()
	go r.bctx.RunIDFetcher(cctx)

	c.Logger().Println("# initialize")

	if err := c.Initialize(); err != nil {
		c.Logger().Printf("initialize に失敗しました.")
		return err
	}

	go bm.endhook(worker)

	bctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	err = worker.Run(bctx, []Task{is})
	if err == context.DeadlineExceeded {
		err = nil
	}
	close(bm.done)

	bm.logger.Println("# benchmark success")

	return err
}
