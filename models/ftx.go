package models

type FTXFuture struct {
	Result []FTXPremium `json:"result"`
}

type FTXPremium struct {
	Future string  `json:"future"`
	Rate   float64 `json:"rate"`
}
