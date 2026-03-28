package metrics

type ClientCategory string

const (
	CategoryNew     ClientCategory = "new"
	CategoryRegular ClientCategory = "regular"
	CategoryTrusted ClientCategory = "trusted"
	CategoryAtRisk  ClientCategory = "at_risk"
)
