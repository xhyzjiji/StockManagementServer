// Package stockmath 实现库存金额计算：金额四舍五入保留两位小数（元到分）。
// Package stockmath implements stock value math; money is rounded half-up to 2 decimals (cents).
package stockmath

import "math"

// InPricing 入库时老库存尚未定价（有库存但总价为 0）的定价方式。
// InPricing is how to price an inbound batch when existing stock has no value yet.
type InPricing string

const (
	// Weighted 本次入库总价摊到全部库存（未定价的老库存相当于按 0 元计价）
	// Weighted spreads this batch's total over all stock (unpriced old stock counts as 0).
	Weighted InPricing = "WEIGHTED"
	// FirstPrice 按本次入库的单价给整批库存（含老库存）统一定价
	// FirstPrice reprices the whole stock (old stock included) at this batch's unit price.
	FirstPrice InPricing = "FIRST_PRICE"
	// Correct 手动修正老库存总价后，再与本次入库正常加权
	// Correct uses a manually corrected old-stock total, then weights normally.
	Correct InPricing = "CORRECT"
)

func ParsePricing(s string) (InPricing, bool) {
	switch InPricing(s) {
	case "", Weighted:
		return Weighted, true
	case FirstPrice, Correct:
		return InPricing(s), true
	}
	return "", false
}

// Epsilon 浮点数量视为 0 的阈值（出清判定）。
// Epsilon is the threshold below which a quantity counts as zero (stock cleared).
const Epsilon = 1e-9

type ValueAfter struct {
	TotalValue float64
	AvgPrice   float64
}

// Round2 HALF_UP 保留两位小数；加微小偏移吸收 1.005*100=100.4999… 这类二进制误差。
// Round2 rounds half-up to 2 decimals; a tiny bias absorbs binary artifacts like 1.005*100=100.4999….
func Round2(v float64) float64 {
	r := math.Floor(math.Abs(v)*100 + 0.5 + 1e-7)
	if r == 0 {
		return 0
	}
	return math.Copysign(r/100, v)
}

// AfterStockIn 剩余总价累加本次入库总价，平均价 = 剩余总价 / 入库后库存。
// AfterStockIn adds the batch total to the stock value; avg = total / quantity after.
func AfterStockIn(beforeQty, beforeValue, inQty, inTotal float64) ValueAfter {
	total := Round2(beforeValue + inTotal)
	qty := beforeQty + inQty
	avg := 0.0
	if qty > 0 {
		avg = Round2(total / qty)
	}
	return ValueAfter{TotalValue: total, AvgPrice: avg}
}

// AfterStockInFirstPrice 按本次入库单价给整批库存重新定价。
// AfterStockInFirstPrice reprices all stock at this batch's unit price.
func AfterStockInFirstPrice(beforeQty, inQty, inTotal float64) ValueAfter {
	avg := Round2(inTotal / inQty)
	return ValueAfter{TotalValue: Round2(avg * (beforeQty + inQty)), AvgPrice: avg}
}

// AfterStockOut 均价不变，剩余总价 = 出库后数量 × 均价；出清时总价与均价归零。
// AfterStockOut keeps the average; value = remaining qty × avg; both reset to 0 once cleared.
func AfterStockOut(afterQty, avgPrice float64) ValueAfter {
	if afterQty < Epsilon {
		return ValueAfter{}
	}
	return ValueAfter{TotalValue: Round2(afterQty * avgPrice), AvgPrice: avgPrice}
}
