package bench

import (
	"fmt"
	"io"
	"log"
	"time"
)

type timeWriter struct {
	out   io.Writer
	start time.Time
}

func (w *timeWriter) Write(p []byte) (n int, err error) {
	d := fmt.Sprintf("[%.5f] ", time.Now().Sub(w.start).Seconds())
	n, err = w.out.Write([]byte(d))
	if err != nil {
		return n, err
	}
	var m int
	m, err = w.out.Write(p)
	n += m
	return
}

func NewLogger(out io.Writer) *log.Logger {
	return log.New(&timeWriter{out, time.Now()}, "", log.LstdFlags|log.Lmicroseconds)
}
