package stockmath

import "testing"

func TestRound2(t *testing.T) {
	cases := map[float64]float64{
		2.675:  2.68,
		1.005:  1.01,
		6.6666: 6.67,
		10:     10,
		-1.005: -1.01,
		0.004:  0,
	}
	for in, want := range cases {
		if got := Round2(in); got != want {
			t.Errorf("Round2(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestAfterStockIn(t *testing.T) {
	got := AfterStockIn(0, 0, 3, 20)
	if got.TotalValue != 20 || got.AvgPrice != 6.67 {
		t.Fatalf("unexpected %+v", got)
	}
	got = AfterStockIn(3, 20, 2, 10)
	if got.TotalValue != 30 || got.AvgPrice != 6 {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestAfterStockInFirstPrice(t *testing.T) {
	got := AfterStockInFirstPrice(5, 5, 25)
	if got.AvgPrice != 5 || got.TotalValue != 50 {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestAfterStockOut(t *testing.T) {
	got := AfterStockOut(2, 6.67)
	if got.TotalValue != 13.34 || got.AvgPrice != 6.67 {
		t.Fatalf("unexpected %+v", got)
	}
	if got := AfterStockOut(1e-12, 6.67); got.TotalValue != 0 || got.AvgPrice != 0 {
		t.Fatalf("cleared stock should reset value, got %+v", got)
	}
}

func TestParsePricing(t *testing.T) {
	if p, ok := ParsePricing(""); !ok || p != Weighted {
		t.Fatal("empty pricing should default to WEIGHTED")
	}
	if _, ok := ParsePricing("bogus"); ok {
		t.Fatal("bogus pricing should be rejected")
	}
}
