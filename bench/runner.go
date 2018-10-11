package bench

import (
	"context"
	"log"
	"time"

	"github.com/ken39arg/isucon2018-final/bench/portal"
	"github.com/ken39arg/isucon2018-final/bench/taskworker"
	"github.com/pkg/errors"
)

type Runner struct {
	mgr   *Manager
	done  chan struct{}
	start time.Time
	end   time.Time
	fail  bool
}

func NewRunner(mgr *Manager) *Runner {
	return &Runner{
		mgr:  mgr,
		done: make(chan struct{}),
	}
}

func (r *Runner) Result() portal.BenchResult {
	score := r.mgr.TotalScore()
	if r.fail {
		// failしていたらmanagerによらずスコアは0とする
		score = 0
	}
	level := r.mgr.GetLevel()
	errors := r.mgr.GetErrorsString()
	r.mgr.Logger().Printf("Score: %d, (level: %d, errors: %d, users: %d/%d)", score, level, r.mgr.ErrorCount(), r.mgr.ActiveInvestors(), r.mgr.AllInvestors())
	scoreboard.Dump()

	logs, _ := r.mgr.GetLogs()
	return portal.BenchResult{
		Pass:      score > 0,
		Score:     score,
		Errors:    errors,
		Logs:      logs,
		LoadLevel: int(level),

		StartTime: r.start,
		EndTime:   r.end,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	m := r.mgr
	defer func() {
		r.end = time.Now()
	}()
	r.start = time.Now()

	cctx, ccancel := context.WithCancel(ctx)
	defer ccancel()
	go m.RunIDFetcher(cctx)

	m.Logger().Println("# initialize")
	if err := m.Initialize(cctx); err != nil {
		return errors.Wrap(err, "Initialize に失敗しました")
	}

	m.Logger().Println("# pre test")
	if err := m.PreTest(cctx); err != nil {
		return errors.Wrap(err, "負荷走行前のテストに失敗しました")
	}

	m.Logger().Printf("# benchmark")
	if err := r.runBenchmark(cctx); err != nil {
		return errors.Wrap(err, "負荷走行 に失敗しました")
	}

	if r.fail {
		return errors.New("finish by fail")
	}

	// cancelたちが終わるように少し待つ(すべての状態管理はつらすぎるので)
	time.Sleep(50 * time.Millisecond)

	m.Logger().Printf("# post test")
	if err := m.PostTest(cctx); err != nil {
		r.fail = true
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

	wc, cancel := context.WithTimeout(ctx, BenchMarkTime)
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
			switch errors.Cause(err) {
			case nil:
				r.mgr.AddScore(task.Score())
			case context.DeadlineExceeded, context.Canceled:
				log.Printf("[INFO] canceled by %s [score:%d]", err, task.Score())
				r.mgr.AddScore(task.Score())
			case ErrAlreadyRetired:
			default:
				r.mgr.Logger().Printf("error: %s", err)
				if e := r.mgr.AppendError(err); e != nil {
					r.fail = true
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
		case <-time.After(TickerInterval):
			// nextが終わってから次のloopとしたいのでtickerではない
			tasks, err := r.mgr.Next()
			if err != nil {
				r.fail = true
				r.mgr.Logger().Printf("エラーのためベンチマークを終了します: %s", err)
				worker.Finish()
			}
			if tasks != nil {
				worker.AddTasks(tasks)
			}
		}
	}
}
