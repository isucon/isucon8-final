package bench

import "time"

const (
	// Timeouts
	ClientTimeout = 10 * time.Second
	InitTimeout   = 10 * time.Second

	// Scores
	GetTradesScore      = 1
	PostBuyOrdersScore  = 5
	PostSellOrdersScore = 5
	GetBuyOrdersScore   = 1
	GetSellOrdersScore  = 1
	TradeSuccessScore   = 10
)
