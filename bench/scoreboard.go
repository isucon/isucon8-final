package bench

import (
	"log"
	"sync"
)

var (
	scoreboard = &ScoreBoard{
		score: make(map[string]int64, 20),
		count: make(map[string]int64, 20),
	}
)

type ScoreBoard struct {
	score map[string]int64
	count map[string]int64
	mux   sync.Mutex
}

func (sb *ScoreBoard) Add(p string, s int64) {
	sb.mux.Lock()
	defer sb.mux.Unlock()
	if _, ok := sb.score[p]; !ok {
		sb.score[p] = 0
		sb.count[p] = 0
	}
	sb.score[p] += s
	sb.count[p]++
}

func (sb *ScoreBoard) Dump() {
	for key, score := range sb.score {
		log.Printf("%s\t: score=%d, count=%d", key, score, sb.count[key])
	}
}
