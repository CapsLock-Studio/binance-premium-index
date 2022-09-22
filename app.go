package main

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type BinancePremium struct {
	Symbol               string `json:"symbol"`
	MarkPrice            string `json:"markPrice"`
	IndexPrice           string `json:"indexPrice"`
	EstimatedSettlePrice string `json:"estimatedSettlePrice"`
	LastFundingRate      string `json:"lastFundingRate"`
	InterestRate         string `json:"interestRate"`
	NextFundingTime      int    `json:"nextFundingTime"`
	Time                 int    `json:"time"`
}

type BinanceHedge struct {
	Symbol         string           `json:"symbol"`
	FundingRateGap float64          `json:"fundingRateGap"`
	MarkPriceGap   float64          `json:"markPriceGap"`
	ProfitForTimes int64            `json:"profitForTimes"`
	Direction      bool             `json:"direction"`
	Index          []BinancePremium `json:"index"`
}

type SortBinanceHedge []BinanceHedge

func (value SortBinanceHedge) Len() int { return len(value) }
func (value SortBinanceHedge) Less(i, j int) bool {
	return value[i].FundingRateGap > value[j].FundingRateGap
}
func (value SortBinanceHedge) Swap(i, j int) { value[i], value[j] = value[j], value[i] }

func extractPremiumIndex(premium []BinancePremium, currency string) (value *BinancePremium) {
	for _, index := range premium {
		if strings.HasSuffix(index.Symbol, currency) {
			value = &index
			break
		}
	}

	return
}

func main() {
	route := gin.Default()

	route.GET("/", func(ctx *gin.Context) {
		res, err := http.Get("https://fapi.binance.com/fapi/v1/premiumIndex")
		if err != nil {
			log.Fatal(err)
		}
		defer res.Body.Close()

		target := make([]BinancePremium, 0)
		decoder := json.NewDecoder(res.Body)
		decoder.Decode(&target)

		currencies := []string{
			"USDT",
			"BUSD",
		}

		// mapping
		mapping := make(map[string][]BinancePremium)
		for _, v := range target {

			re := regexp.MustCompile("(" + strings.Join(currencies, "|") + ")$")

			if !re.Match([]byte(v.Symbol)) {
				continue
			}

			index := re.ReplaceAllString(v.Symbol, "")

			if mapping[index] == nil {
				mapping[index] = make([]BinancePremium, 0)
			}

			mapping[index] = append(mapping[index], v)
		}

		// aggregate mapping
		result := make([]BinanceHedge, 0)
		for i, v := range mapping {
			if len(v) == 2 {
				indexUSDT := extractPremiumIndex(v, "USDT")
				indexBUSD := extractPremiumIndex(v, "BUSD")
				rateUSDT, _ := decimal.NewFromString(indexUSDT.LastFundingRate)
				rateBUSD, _ := decimal.NewFromString(indexBUSD.LastFundingRate)
				markPriceUSDT, _ := decimal.NewFromString(indexUSDT.MarkPrice)
				markPriceBUSD, _ := decimal.NewFromString(indexBUSD.MarkPrice)

				fundinRateGap, _ := rateUSDT.
					Sub(rateBUSD).
					Mul(decimal.NewFromInt(100)).
					Abs().
					Float64()

				markPriceGapValue := markPriceUSDT.Sub(markPriceBUSD)

				markPriceGap, _ := markPriceGapValue.
					Div(markPriceUSDT).
					Mul(decimal.NewFromInt(100)).
					Abs().
					Float64()

				hedge := BinanceHedge{
					Symbol:         i,
					FundingRateGap: fundinRateGap,
					MarkPriceGap:   markPriceGap,
					Index:          v,
					Direction:      rateUSDT.GreaterThan(rateBUSD),
				}

				if fundinRateGap > 0 {
					profitForTimes := decimal.
						NewFromFloat(markPriceGap).
						Div(decimal.NewFromFloat(fundinRateGap)).
						Ceil().
						IntPart()

					hedge.ProfitForTimes = profitForTimes
				}

				result = append(result, hedge)
			}
		}

		sort.Sort(SortBinanceHedge(result))

		ctx.JSON(http.StatusOK, result)
	})

	route.Run()
}
