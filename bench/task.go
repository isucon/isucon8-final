package bench

import (
	"context"
	"fmt"
)

var ErrNoScore = fmt.Errorf("no score")

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
		if err == ErrNoScore {
			return nil
		}
		return err
	}
	return nil
}

type ListTask struct {
	*taskBase
	tasks []Task
}

func NewListTask(cap int) *ListTask {
	return &ListTask{
		taskBase: &taskBase{score: 0},
		tasks:    make([]Task, 0, cap),
	}
}

func (t *ListTask) Add(task Task) {
	t.tasks = append(t.tasks, task)
}

func (t *ListTask) Run(ctx context.Context) error {
	for _, task := range t.tasks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := task.Run(ctx); err != nil {
				return err
			}
			t.score += task.Score()
		}
	}
	return nil
}
