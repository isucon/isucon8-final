package bench

import (
	"context"
	"fmt"
)

type Task interface {
	Run(context.Context) error
	WriteError(error)
	Error() error
	Score() int64
}

type Worker struct {
	taskCh chan Task
	resCh  chan Task
}

func (w *Worker) Run(ctx context.Context, tasks []Task) error {
	go w.AddTasks(tasks)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case t, ok := <-w.taskCh:
			if !ok {
				return nil
			}
			go func() {
				defer func() {
					if r := recover(); r != nil {
						err, ok := r.(error)
						if !ok {
							err = fmt.Errorf("task panic: %v", r)
						}
						t.WriteError(err)
					}
					w.resCh <- t
				}()
				if err := t.Run(ctx); err != nil {
					t.WriteError(err)
				}
			}()
		}
	}
}

func (w *Worker) AddTasks(tasks []Task) {
	for _, t := range tasks {
		w.taskCh <- t
	}
}

func (w *Worker) AddTask(task Task) {
	w.AddTasks([]Task{task})
}

func (w *Worker) TaskEnd() <-chan Task {
	return w.resCh
}

func (w *Worker) Finish() {
	close(w.taskCh)
}

func NewWorker() *Worker {
	return &Worker{
		taskCh: make(chan Task, 1000),
		resCh:  make(chan Task, 1000),
	}
}
