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

type ScoreBoard struct {
	count map[ScoreType]int64
	mux   sync.Mutex
}

func (sb *ScoreBoard) Add(p ScoreType) {
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
	for i := 0; i < 15; i++ {
		st := ScoreType(i)
		if count, ok := sb.count[st]; ok {
			log.Printf("[INFO] %-16s: score=%d, count=%d", st, count*st.Score(), count)
		}
	}
}

type ScoreMsg struct {
	st  ScoreType
	err error
	sns bool
}
