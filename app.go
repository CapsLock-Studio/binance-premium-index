package main

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/CapsLock-Studio/binance-premium-index/models"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type SortBinanceHedge []models.BinanceHedge

func (value SortBinanceHedge) Len() int { return len(value) }
func (value SortBinanceHedge) Less(i, j int) bool {
	return value[i].FundingRateGap > value[j].FundingRateGap
}
func (value SortBinanceHedge) Swap(i, j int) { value[i], value[j] = value[j], value[i] }

func extractPremiumIndex(premium []models.BinancePremium, currency string) (value *models.BinancePremium) {
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

	route.Use(gin.Recovery())
	route.GET("/", func(ctx *gin.Context) {
		res, err := http.Get("https://fapi.binance.com/fapi/v1/premiumIndex")
		if err != nil {
			log.Fatal(err)
		}
		defer res.Body.Close()

		target := make([]models.BinancePremium, 0)
		decoder := json.NewDecoder(res.Body)
		decoder.Decode(&target)

		currencies := []string{
			"USDT",
			"BUSD",
		}

		// mapping
		mapping := make(map[string][]models.BinancePremium)
		for _, v := range target {

			re := regexp.MustCompile("(" + strings.Join(currencies, "|") + ")$")

			if !re.Match([]byte(v.Symbol)) {
				continue
			}

			index := re.ReplaceAllString(v.Symbol, "")

			if mapping[index] == nil {
				mapping[index] = make([]models.BinancePremium, 0)
			}

			mapping[index] = append(mapping[index], v)
		}

		// aggregate mapping
		result := make([]models.BinanceHedge, 0)
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

				hedge := models.BinanceHedge{
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
