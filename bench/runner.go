package bench

import (
	"context"
	"time"

	"github.com/ken39arg/isucon2018-final/bench/taskworker"
	"github.com/pkg/errors"
)

type Runner struct {
	mgr      *Manager
	timeout  time.Duration
	interval time.Duration
	done     chan struct{}
}

func NewRunner(mgr *Manager, timeout, interval time.Duration) *Runner {
	return &Runner{
		mgr:      mgr,
		timeout:  timeout,
		interval: interval,
		done:     make(chan struct{}),
	}
}

func (r *Runner) Result() {
	c := r.mgr
	c.Logger().Printf("Score: %d, (level: %d, errors: %d, users: %d/%d)", c.TotalScore(), c.level, c.ErrorCount(), c.ActiveInvestors(), c.AllInvestors())
}

func (r *Runner) Run(ctx context.Context) error {
	m := r.mgr

	cctx, ccancel := context.WithCancel(ctx)
	defer ccancel()
	go m.RunIDFetcher(cctx)

	m.Logger().Println("# initialize")
	if err := m.Initialize(); err != nil {
		return errors.Wrap(err, "Initialize に失敗しました")
	}

	m.Logger().Println("# pre test")
	if err := m.PreTest(); err != nil {
		return errors.Wrap(err, "負荷走行前のテストに失敗しました")
	}

	m.Logger().Printf("# benchmark")
	if err := r.runBenchmark(ctx); err != nil {
		return errors.Wrap(err, "負荷走行 に失敗しました")
	}

	m.Logger().Printf("# post test")
	if err := m.PostTest(); err != nil {
		return errors.Wrap(err, "負荷走行後のテストに失敗しました")
	}

	return nil
}

func (r *Runner) runBenchmark(ctx context.Context) error {
	tasks, err := r.mgr.Start()
	if err != nil {
		r.mgr.Logger().Printf("初期化に失敗しました。err:%s", err)
		return err
	}

	worker := taskworker.NewWorker()
	go r.handleWorker(worker)

	go r.runTicker(worker)

	wc, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	err = worker.Run(wc, tasks)
	if err == context.DeadlineExceeded {
		err = nil
	}
	close(r.done)
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
			switch err {
			case context.DeadlineExceeded, nil:
				r.mgr.AddScore(task.Score())
			case ErrAlreadyRetired:
			default:
				r.mgr.Logger().Printf("error: %s", err)
				if e := r.mgr.IncrErr(); e != nil {
					r.mgr.Logger().Printf("ベンチマークを終了します: %s", e)
					worker.Finish()
				}
			}
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
			tasks, err := r.mgr.Next()
			if err != nil {
				r.mgr.Logger().Printf("エラーのためベンチマークを終了します: %s", err)
				worker.Finish()
			}
			if tasks != nil {
				worker.AddTasks(tasks)
			}
		}
	}
}
