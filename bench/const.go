package bench

import "time"

const (
	// Timeouts
	BenchMarkTime  = 60 * time.Second      // 負荷走行の時間
	TickerInterval = 20 * time.Millisecond // tickerのinterval

	InitTimeout   = 30 * time.Second       // Initialize のタイムアウト
	ClientTimeout = 15 * time.Second       // HTTP clientのタイムアウト
	RetireTimeout = 10 * time.Second       // clientが退役するタイムアウト時間
	RetryInterval = 500 * time.Millisecond // 50x系でエラーになったときのretry間隔

	TestTradeTimeout = 5 * time.Second  // testでのtradeは成立までの時間
	LogAllowedDelay  = 10 * time.Second // logの遅延が許される時間

	PollingInterval     = 500 * time.Millisecond // clientのポーリング感覚
	OrderUpdateInterval = 2 * time.Second        // 注文間隔
	BruteForceDelay     = 500 * time.Millisecond // 総当たりログイン試行間隔

	AddUsersOnShare   = 5  // SNSシェアによって増えるユーザー数
	AddUsersOnNatural = 2  // 自然増で増えるユーザー数
	DefaultWorkers    = 10 // 初期
	BruteForceWorkers = 2  // ログインを試行してくるユーザー

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
