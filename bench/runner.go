package bench

import (
	"context"
	"time"
)

type Runner struct {
	bctx     *Context
	timeout  time.Duration
	interval time.Duration
	done     chan struct{}
}

func NewRunner(bctx *Context, timeout, interval time.Duration) *Runner {
	return &Runner{
		bctx:     bctx,
		timeout:  timeout,
		interval: interval,
		done:     make(chan struct{}),
	}
}

func (r *Runner) Result() {
	c := r.bctx
	c.Logger().Printf("Score: %d, (level: %d, errors: %d)", c.GetScore(), c.level, c.ErrorCount())
}

func (r *Runner) Run(ctx context.Context) error {
	c := r.bctx

	cctx, ccancel := context.WithCancel(ctx)
	defer ccancel()
	go c.RunIDFetcher(cctx)

	c.Logger().Println("# initialize")

	tasks, err := c.Start()
	if err != nil {
		c.Logger().Printf("Initialize に失敗しました. err:%s", err)
		return err
	}

	c.Logger().Printf("# benchmark start")
	worker := NewWorker()
	go r.handleWorker(worker)

	go r.runTicker(worker)

	bctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	err = worker.Run(bctx, tasks)
	if err == context.DeadlineExceeded {
		err = nil
	}
	close(r.done)

	c.Logger().Println("# benchmark success")

	// TODO bench終了後N秒経過してからloggerとbankをチェックしたい

	return err
}

func (r *Runner) handleWorker(worker *Worker) {
	ch := worker.TaskEnd()
	for {
		select {
		case <-r.done:
			return
		case task := <-ch:
			err := task.Error()
			if err != nil && err != context.DeadlineExceeded {
				r.bctx.IncrErr()
				r.bctx.Logger().Printf("error: %s", err)
			}
			r.bctx.AddScore(task.Score())
		}
	}
}

func (r *Runner) runTicker(worker *Worker) {
	for {
		select {
		case <-r.done:
			return
		case <-time.After(r.interval):
			// nextが終わってから次のloopとしたいのでtickerではない
			tasks, err := r.bctx.Next()
			if err != nil {
				r.bctx.Logger().Printf("error: %s", err)
			}
			if tasks != nil {
				worker.AddTasks(tasks)
			}
		}
	}
}
