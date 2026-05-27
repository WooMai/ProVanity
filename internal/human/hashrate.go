package human

import (
	"math"
	"strconv"
	"strings"
)

type rateUnit struct {
	unit       string
	multiplier float64
}

var rateUnits = []rateUnit{
	{unit: "H/s", multiplier: 1},
	{unit: "KH/s", multiplier: 1e3},
	{unit: "MH/s", multiplier: 1e6},
	{unit: "GH/s", multiplier: 1e9},
	{unit: "TH/s", multiplier: 1e12},
	{unit: "PH/s", multiplier: 1e15},
}

// FormatHashrate renders an integer hash rate with SI units.
func FormatHashrate(rate uint64) string {
	return FormatHashrateFloat(float64(rate))
}

// FormatHashrateFloat renders a hash rate with SI units.
func FormatHashrateFloat(rate float64) string {
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return "0 H/s"
	}

	unit := 0
	for rate >= 1000 && unit < len(rateUnits)-1 {
		rate /= 1000
		unit++
	}

	precision := 2
	if rate >= 10 {
		precision = 1
	}
	value := strconv.FormatFloat(rate, 'f', precision, 64)
	value = strings.TrimRight(strings.TrimRight(value, "0"), ".")
	return value + " " + rateUnits[unit].unit
}
