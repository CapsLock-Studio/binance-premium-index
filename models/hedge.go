package models

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type BinanceHedge struct {
	Symbol              string           `json:"symbol"`
	FundingRateGap      float64          `json:"fundingRateGap"`
	CorssFundingRateGap float64          `json:"crossFundingRateGap"`
	MarkPriceGap        float64          `json:"markPriceGap"`
	ProfitForTimes      int64            `json:"profitForTimes"`
	Direction           bool             `json:"direction"`
	CrossDirection      bool             `json:"crossDirection"`
	Index               []BinancePremium `json:"index"`
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

func (h *BinanceHedge) GetLeftMinutes(currency string) (minutes float64) {
	for _, i := range h.Index {
		if strings.HasSuffix(i.Symbol, currency) {
			nextFundingTime := time.Unix(0, int64(i.NextFundingTime)*int64(time.Millisecond))
			now := time.Unix(0, int64(i.Time)*int64(time.Millisecond))

			minutes = nextFundingTime.Sub(now).Minutes()
			break
		}
	}

	return
}
