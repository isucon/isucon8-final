package bench

import "context"

type taskBase struct {
	score int64
	err   error
}

func (t *taskBase) WriteError(err error) {
	t.err = err
}

func (t *taskBase) Error() error {
	return t.err
}

func (t *taskBase) Score() int64 {
	if t.err != nil && t.err != context.DeadlineExceeded {
		return 0
	}
	return t.score
}

type ExecTask struct {
	*taskBase
	runner func(context.Context) error
}

func NewExecTask(runner func(context.Context) error, score int64) *ExecTask {
	return &ExecTask{
		taskBase: &taskBase{score: score},
		runner:   runner,
	}
}

func (t *ExecTask) Run(ctx context.Context) error {
	if err := t.runner(ctx); err != nil {
		t.score = 0
		return err
	}
	return nil
}
