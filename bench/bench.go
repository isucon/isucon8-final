package bench

import (
	"context"
	"io"
	"log"
	"net/url"
	"time"
)

type BenchLevel struct {
	// Level Upに必要なtask完走数
	Next int
	// LevelUpで投入されるWorker数
	Worker int
	// PlayerScenarioに渡される引数
	PageCount, PageLoop, PlayLoop int
	// ScaleupScenarioに渡される引数
	TestCount, TestPage, TestLoop int
	// スコアリング係数
	Factor float64
}

var BenchLevelTable = []BenchLevel{
	{Next: 1, Worker: 1, Factor: 1}, // Init
	{Next: 10, Worker: 10, PageCount: 10, PageLoop: 1, PlayLoop: 1, Factor: 1.5, TestCount: 30, TestPage: 3, TestLoop: 3},
	{Next: 30, Worker: 10, PageCount: 15, PageLoop: 1, PlayLoop: 2, Factor: 1.8, TestCount: 40, TestPage: 3, TestLoop: 3},
	{Next: 60, Worker: 20, PageCount: 20, PageLoop: 2, PlayLoop: 2, Factor: 2.2, TestCount: 60, TestPage: 4, TestLoop: 3},
	{Next: 100, Worker: 30, PageCount: 30, PageLoop: 3, PlayLoop: 2, Factor: 3.0, TestCount: 100, TestPage: 5, TestLoop: 3},
	{Next: 500, Worker: 10, PageCount: 50, PageLoop: 3, PlayLoop: 3, Factor: 4.0, TestCount: 100, TestPage: 6, TestLoop: 3},
}

var ScrollFactor = 1.0

type BenchmarkerParams struct {
	Domain string
	Time   time.Duration
}

func (bp BenchmarkerParams) Validate() error {
	if _, err := url.ParseRequestURI(bp.Domain); err != nil {
		return err
	}
	return nil
}

type BenchmarkerResult struct {
	Level      int
	Score      int64
	ErrorCount int
}

type Benchmarker struct {
	logger  *log.Logger
	done    chan struct{}
	success int
	params  BenchmarkerParams
	result  BenchmarkerResult
}

func (bm *Benchmarker) endhook(worker *Worker) {
	ch := worker.TaskEnd()
	for {
		select {
		case <-bm.done:
			return
		case task := <-ch:
			err := task.Error()
			if err != nil && err != context.DeadlineExceeded {
				bm.result.ErrorCount++
				bm.logger.Printf("error: %s", err)
			}
			score := task.Score()
			if 0 < score {
				bm.result.Score += score
				bm.success++
			}
			if err == context.DeadlineExceeded {
				continue
			}
			switch s := task.(type) {
			case *InitializeScenario:
				if task.Error() != nil {
					bm.logger.Printf("Initializeに失敗したので終了します")
					worker.Finish()
				} else {
					tsks, err := bm.LevelUp()
					if err != nil {
						bm.logger.Printf("Fatal: %s", err)
						return
					}
					bm.logger.Printf("Initialize完了 %d", bm.result.Level)
					worker.AddTasks(tsks)
					scroll, err := NewScrollScenarioScenario(bm.params.Domain, ScrollFactor, 100)
					if err != nil {
						bm.logger.Printf("Fatal: %s", err)
						return
					}
					worker.AddTask(scroll)
				}
			case *ScaleupScenario:
				if task.Error() != nil {
					bm.logger.Println("Worker Levelを上げられませんでした")
				} else {
					tsks, err := bm.LevelUp()
					if err != nil {
						bm.logger.Printf("Fatal: %s", err)
						return
					}
					bm.logger.Printf("Worker Levelが上がります %d", bm.result.Level)
					worker.AddTasks(tsks)
				}
			case *PlayerScenario:
				lv := bm.Level()
				nt, err := NewPlayerScenario(bm.params.Domain, lv.Factor, lv.PageCount, lv.PageLoop, lv.PlayLoop)
				if err != nil {
					bm.logger.Printf("Fatal: %s", err)
					return
				}
				//bm.logger.Printf("タスクを補充 success: %d", bm.success)
				worker.AddTask(nt)
			case *ScrollScenario:
				scroll, err := NewScrollScenarioScenario(bm.params.Domain, ScrollFactor, s.Count()+100)
				if err != nil {
					bm.logger.Printf("Fatal: %s", err)
					return
				}
				//bm.logger.Printf("タスクを補充 success: %d", bm.success)
				worker.AddTask(scroll)
			}
			if lv := bm.Level(); 0 < score && lv.Next == bm.success {
				nt, err := NewScaleupScenario(bm.params.Domain, lv.Factor, lv.TestCount, lv.TestPage, lv.TestLoop)
				if err != nil {
					bm.logger.Printf("Fatal: %s", err)
					return
				}
				bm.logger.Println("Worker Level 上昇に挑戦します")
				worker.AddTask(nt)
			}
		}
	}
}

func (bm *Benchmarker) Run(ctx context.Context) error {
	worker := NewWorker()

	bm.logger.Println("# benchmark start")

	is, err := NewInitializeScenario(bm.params.Domain, bm.Level().Factor)
	if err != nil {
		return err
	}

	go bm.endhook(worker)

	bctx, cancel := context.WithTimeout(ctx, bm.params.Time)
	defer cancel()
	err = worker.Run(bctx, []Task{is})
	if err == context.DeadlineExceeded {
		err = nil
	}
	close(bm.done)

	bm.logger.Println("# benchmark success")

	return err
}

func (bm *Benchmarker) Result() {
	bm.logger.Printf("level: %d, score: %d, errors: %d, success: %d", bm.result.Level, bm.result.Score, bm.result.ErrorCount, bm.success)
}

func (bm *Benchmarker) Score() int64 {
	return bm.result.Score
}

func (bm *Benchmarker) LoadLevel() int {
	return bm.result.Level
}

func (bm *Benchmarker) LevelUp() (tasks []Task, err error) {
	if bm.result.Level < len(BenchLevelTable) {
		bm.result.Level++
		lv := bm.Level()
		tasks = make([]Task, lv.Worker)
		for i := range tasks {
			tasks[i], err = NewPlayerScenario(bm.params.Domain, lv.Factor, lv.PageCount, lv.PageLoop, lv.PlayLoop)
			if err != nil {
				return nil, err
			}
		}
	}
	return
}

func (bm *Benchmarker) Level() BenchLevel {
	return BenchLevelTable[bm.result.Level]
}

func NewBenchmarker(out io.Writer, params BenchmarkerParams) (*Benchmarker, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	return &Benchmarker{
		logger: NewLogger(out),
		done:   make(chan struct{}),
		params: params,
		result: BenchmarkerResult{},
	}, nil
}
