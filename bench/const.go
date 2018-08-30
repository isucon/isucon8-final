package bench

import "time"

const (
	// Timeouts
	ClientTimeout = 10 * time.Second
	InitTimeout   = 10 * time.Second

	AddWorkersByLevel = 10

	// Scores
	SignupScore       = 1
	SigninScore       = 1
	GetTradesScore    = 1
	PostOrdersScore   = 5
	GetOrdersScore    = 1
	DeleteOrdersScore = 3
	TradeSuccessScore = 10
	GetInfoScore      = 1
	GetTopScore       = 1

	// error
	AllowErrorMin = 10 // levelによらずここまでは許容範囲というエラー数
	AllowErrorMax = 50 // levelによらずこれ以上は許さないというエラー数
)
