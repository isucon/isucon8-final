package bench

import "time"

const (
	// Timeouts
	ClientTimeout = 10 * time.Second // HTTP clientのタイムアウト
	InitTimeout   = 10 * time.Second // Initialize のタイムアウト

	RetireTimeout = 5 * time.Second        // clientが退役するタイムアウト時間
	RetryInterval = 500 * time.Millisecond // 50x系でエラーになったときのretry間隔

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
