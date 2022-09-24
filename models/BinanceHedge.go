package models

type BinanceHedge struct {
	Symbol         string           `json:"symbol"`
	FundingRateGap float64          `json:"fundingRateGap"`
	MarkPriceGap   float64          `json:"markPriceGap"`
	ProfitForTimes int64            `json:"profitForTimes"`
	Direction      bool             `json:"direction"`
	Index          []BinancePremium `json:"index"`
}
