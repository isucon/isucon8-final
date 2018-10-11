package bench

import (
	"fmt"
	"log"
	"sync"
)

type ScoreType int

const (
	ScoreTypeGetTop ScoreType = 1 + iota
	ScoreTypeSignup
	ScoreTypeSignin
	ScoreTypeGetInfo
	ScoreTypePostOrders
	ScoreTypeGetOrders
	ScoreTypeDeleteOrders
	ScoreTypeTradeSuccess
)

func (st ScoreType) String() string {
	switch st {
	case ScoreTypeGetTop:
		return "GetTop"
	case ScoreTypeSignup:
		return "Signup"
	case ScoreTypeSignin:
		return "Signin"
	case ScoreTypeGetInfo:
		return "GetInfo"
	case ScoreTypeGetOrders:
		return "GetOrders"
	case ScoreTypePostOrders:
		return "PostOrders"
	case ScoreTypeDeleteOrders:
		return "DeleteOrders"
	case ScoreTypeTradeSuccess:
		return "TradeSuccess"
	default:
		return fmt.Sprintf("Unknown[%d]", st)
	}
}

func (st ScoreType) Score() int64 {
	switch st {
	case ScoreTypeGetTop:
		return GetTopScore
	case ScoreTypeSignup:
		return SignupScore
	case ScoreTypeSignin:
		return SigninScore
	case ScoreTypeGetInfo:
		return GetInfoScore
	case ScoreTypeGetOrders:
		return GetOrdersScore
	case ScoreTypePostOrders:
		return PostOrdersScore
	case ScoreTypeDeleteOrders:
		return DeleteOrdersScore
	case ScoreTypeTradeSuccess:
		return TradeSuccessScore
	default:
		log.Printf("[WARN] not defined score [%d]", st)
		return 0
	}
}

var (
	scoreboard = &ScoreBoard{
		count: make(map[ScoreType]int64, 20),
	}
)

type ScoreBoard struct {
	count map[ScoreType]int64
	mux   sync.Mutex
}

func (sb *ScoreBoard) Add(p ScoreType, s int64) {
	sb.mux.Lock()
	defer sb.mux.Unlock()
	if _, ok := sb.count[p]; !ok {
		sb.count[p] = 0
	}
	sb.count[p]++
}

func (sb *ScoreBoard) Dump() {
	sb.mux.Lock()
	defer sb.mux.Unlock()
	for st, count := range sb.count {
		log.Printf("%s\t: score=%d, count=%d", st, count*st.Score(), count)
	}
}

type ScoreMsg struct {
	st  ScoreType
	err error
	sns bool
}
