package taskworker

import (
	"context"
	"fmt"
)

var ErrNoScore = fmt.Errorf("no score")

type Task interface {
	Run(context.Context) error
	WriteError(error)
	Error() error
	Score() int64
}

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

type scoreTask struct {
	*taskBase
	runner func(context.Context) (int64, error)
}

func (t *scoreTask) Run(ctx context.Context) error {
	var err error
	if t.score, err = t.runner(ctx); err != nil {
		t.score = 0
		if err == ErrNoScore {
			return nil
		}
		return err
	}
	return nil
}

func NewScoreTask(runner func(context.Context) (int64, error)) Task {
	return &scoreTask{
		taskBase: &taskBase{},
		runner:   runner,
	}
}

func NewExecTask(runner func(context.Context) error, score int64) Task {
	return NewScoreTask(func(ctx context.Context) (int64, error) {
		return score, runner(ctx)
	})
}

type SerialTask struct {
	*taskBase
	tasks []Task
}

func NewSerialTask(cap int) *SerialTask {
	return &SerialTask{
		taskBase: &taskBase{},
		tasks:    make([]Task, 0, cap),
	}
}

func (t *SerialTask) Add(task Task) {
	t.tasks = append(t.tasks, task)
}

func (t *SerialTask) Run(ctx context.Context) error {
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
