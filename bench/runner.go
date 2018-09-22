package bench

import (
	"context"
	"time"

	"github.com/ken39arg/isucon2018-final/bench/taskworker"
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
	c.Logger().Printf("Score: %d, (level: %d, errors: %d, users: %d/%d)", c.TotalScore(), c.level, c.ErrorCount(), c.ActiveInvestors(), c.AllInvestors())
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
	worker := taskworker.NewWorker()
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

func (r *Runner) handleWorker(worker *taskworker.Worker) {
	ch := worker.TaskEnd()
	for {
		select {
		case <-r.done:
			return
		case task := <-ch:
			err := task.Error()
			if err != nil && err != context.DeadlineExceeded {
				r.bctx.Logger().Printf("error: %s", err)
				if e := r.bctx.IncrErr(); e != nil {
					r.bctx.Logger().Printf("ベンチマークを終了します: %s", e)
					worker.Finish()
				}
			}
			r.bctx.AddScore(task.Score())
		}
	}
}

func (r *Runner) runTicker(worker *taskworker.Worker) {
	for {
		select {
		case <-r.done:
			return
		case <-time.After(r.interval):
			// nextが終わってから次のloopとしたいのでtickerではない
			tasks, err := r.bctx.Next()
			if err != nil {
				r.bctx.Logger().Printf("エラーのためベンチマークを終了します: %s", err)
				worker.Finish()
			}
			if tasks != nil {
				worker.AddTasks(tasks)
			}
		}
	}
}
