package taskworker

import (
	"context"
	"fmt"
	"log"
	"runtime"
)

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
						log.Printf("[WARN] %s", err)
						for depth := 0; depth < 10; depth++ {
							_, file, line, ok := runtime.Caller(depth)
							if !ok {
								break
							}
							log.Printf("======> %d: %v:%d", depth, file, line)
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
	defer func() {
		if e := recover(); e != nil {
			log.Printf("[WARN] maybe multi close. e:%s", e)
		}
	}()
	close(w.taskCh)
}

func NewWorker() *Worker {
	return &Worker{
		taskCh: make(chan Task, 1000),
		resCh:  make(chan Task, 1000),
	}
}
