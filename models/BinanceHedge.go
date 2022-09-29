package models

import (
	"strings"

	"github.com/shopspring/decimal"
)

type BinanceHedge struct {
	Symbol         string           `json:"symbol"`
	FundingRateGap float64          `json:"fundingRateGap"`
	MarkPriceGap   float64          `json:"markPriceGap"`
	ProfitForTimes int64            `json:"profitForTimes"`
	Direction      bool             `json:"direction"`
	Index          []BinancePremium `json:"index"`
}

func (h *BinanceHedge) GetPrice(currency string) (price float64) {
	for _, i := range h.Index {
		if strings.HasSuffix(i.Symbol, currency) {
			markPrice, _ := decimal.NewFromString(i.MarkPrice)
			price, _ = markPrice.Float64()
			break
		}
	}

	return
}
