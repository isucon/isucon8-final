package bench

import "time"

const (
	// Timeouts
	BenchMarkTime  = 60 * time.Second      // 負荷走行の時間
	TickerInterval = 20 * time.Millisecond // tickerのinterval

	ClientTimeout = 10 * time.Second // HTTP clientのタイムアウト
	InitTimeout   = 30 * time.Second // Initialize のタイムアウト

	RetireTimeout = 5 * time.Second        // clientが退役するタイムアウト時間
	RetryInterval = 500 * time.Millisecond // 50x系でエラーになったときのretry間隔

	TestTradeTimeout = 5 * time.Second  // testでのtradeは成立までの時間
	LogAllowedDelay  = 10 * time.Second // logの遅延が許される時間

	PollingInterval     = 500 * time.Millisecond // clientのポーリング感覚
	OrderUpdateInterval = 2 * time.Second        // 注文間隔

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
