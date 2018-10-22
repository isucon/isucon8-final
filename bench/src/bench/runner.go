package bench

import (
	"context"
	"time"

	"bench/portal"
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
	score := r.mgr.FinalScore()
	if r.fail {
		// failしていたらmanagerによらずスコアは0とする
		score = 0
	}
	level := r.mgr.GetLevel()
	errors := r.mgr.GetErrorsString()
	if score > 0 {
		r.mgr.Logger().Printf("Pass => Score: %d, (level: %d, errors: %d, users: %d/%d)", score, level, r.mgr.ErrorCount(), r.mgr.ActiveUsers(), r.mgr.AllUsers())
	} else {
		r.mgr.Logger().Printf("Fail => Score: %d, (level: %d, errors: %d, users: %d/%d, score:%d)", score, level, r.mgr.ErrorCount(), r.mgr.ActiveUsers(), r.mgr.AllUsers(), r.mgr.TotalScore())
	}

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

	if err := r.runScenarioBenchmark(cctx); err != nil {
		r.fail = true
		return errors.Wrap(err, "負荷走行 に失敗しました")
	}
	m.scoreboard.Dump()

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

func (r *Runner) runScenarioBenchmark(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, BenchMarkTime)
	defer cancel()

	err := r.mgr.ScenarioStart(cctx)
	if err == context.DeadlineExceeded {
		err = nil
	}
	return err
}
