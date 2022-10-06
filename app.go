package main

import (
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/CapsLock-Studio/binance-premium-index/models"
	"github.com/gin-gonic/gin"
	"github.com/parnurzeal/gorequest"
	"github.com/shopspring/decimal"
)

type SortBinanceHedge []models.BinanceHedge

func (value SortBinanceHedge) Len() int { return len(value) }
func (value SortBinanceHedge) Less(i, j int) bool {
	return value[i].FundingRateGap > value[j].FundingRateGap
}
func (value SortBinanceHedge) Swap(i, j int) { value[i], value[j] = value[j], value[i] }

type SortCrossBinanceHedge []models.BinanceHedge

func (value SortCrossBinanceHedge) Len() int { return len(value) }
func (value SortCrossBinanceHedge) Less(i, j int) bool {
	return value[i].CorssFundingRateGap > value[j].CorssFundingRateGap
}
func (value SortCrossBinanceHedge) Swap(i, j int) { value[i], value[j] = value[j], value[i] }

type Request struct {
	Cross bool `form:"cross"`
}

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

	route.GET("/", func(ctx *gin.Context) {
		var r Request
		if err := ctx.Bind(&r); err != nil {
			return
		}

		binancePremium := make([]models.BinancePremium, 0)
		ftxPremium := models.FTXFuture{}

		wg := &sync.WaitGroup{}

		wg.Add(1)
		go func() {
			defer wg.Done()

			gorequest.
				New().
				Get("https://fapi.binance.com/fapi/v1/premiumIndex").
				EndStruct(&binancePremium)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			gorequest.
				New().
				Get("https://ftx.com/api/funding_rates").
				EndStruct(&ftxPremium)
		}()

		wg.Wait()

		currencies := []string{
			"USDT",
			"BUSD",
		}

		// mapping
		mapping := make(map[string][]models.BinancePremium)
		for _, v := range binancePremium {

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

				hedge.FundingInterest, _ = decimal.
					NewFromFloat(fundinRateGap).
					Mul(decimal.NewFromInt(3)).
					Mul(decimal.NewFromInt(365)).
					Div(decimal.NewFromInt(2)).
					Float64()

				for _, r := range ftxPremium.Result {
					regexK := regexp.MustCompile("^K")
					symbol := regexK.ReplaceAllString(r.Future, "1000")

					regexPerp := regexp.MustCompile("-PERP")
					symbol = regexPerp.ReplaceAllLiteralString(symbol, "")

					regexUni := regexp.MustCompile("^UNISWAP$")
					symbol = regexUni.ReplaceAllString(symbol, "UNI")

					if symbol == i {
						rate := decimal.
							NewFromFloat(r.Rate).
							Mul(decimal.NewFromInt(8)).
							Mul(decimal.NewFromInt(100))

						crossUSDT, _ := rate.
							Sub(rateUSDT).
							Abs().
							Float64()

						crossBUSD, _ := rate.
							Sub(rateBUSD).
							Abs().
							Float64()

						crossRate := math.Max(crossBUSD, crossUSDT)

						if crossRate == crossBUSD {
							hedge.CrossDirection = rate.GreaterThan(rateBUSD)
						}

						if crossRate == crossUSDT {
							hedge.CrossDirection = rate.GreaterThan(rateUSDT)
						}

						hedge.CorssFundingRateGap = crossRate
						hedge.CorssFundingInterest, _ = decimal.
							NewFromFloat(crossRate).
							Mul(decimal.NewFromInt(3)).
							Mul(decimal.NewFromInt(365)).
							Div(decimal.NewFromInt(2)).
							Float64()

						break
					}
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

		if r.Cross {
			sort.Sort(SortCrossBinanceHedge(result))
		} else {
			sort.Sort(SortBinanceHedge(result))
		}

		ctx.JSON(http.StatusOK, result)
	})

	route.Run()
}
