package estimate

import (
	"fmt"
	"math"
	"time"

	"github.com/woomai/provanity/internal/vanity"
)

type Estimate struct {
	Probability float64
	Expected    time.Duration
	P50         time.Duration
	P95         time.Duration
}

type NextScoreRemaining struct {
	Score     int
	Quantile  string
	Total     time.Duration
	Remaining time.Duration
	Overdue   bool
}

func ForPattern(pattern vanity.Pattern, hashrate float64) (Estimate, error) {
	if hashrate <= 0 {
		return Estimate{}, fmt.Errorf("hashrate must be greater than zero")
	}
	probability, err := Probability(pattern)
	if err != nil {
		return Estimate{}, err
	}

	denominator := probability * hashrate
	return Estimate{
		Probability: probability,
		Expected:    secondsDuration(1 / denominator),
		P50:         secondsDuration(math.Log(2) / denominator),
		P95:         secondsDuration(math.Log(20) / denominator),
	}, nil
}

func ForNextScoreBase(currentScore, alphabetSize int, hashrate float64, elapsedSinceBest time.Duration) (NextScoreRemaining, error) {
	if currentScore < 0 {
		return NextScoreRemaining{}, fmt.Errorf("current score cannot be negative")
	}
	if alphabetSize <= 1 {
		return NextScoreRemaining{}, fmt.Errorf("alphabet size must be greater than one")
	}
	if hashrate <= 0 {
		return NextScoreRemaining{}, fmt.Errorf("hashrate must be greater than zero")
	}
	if elapsedSinceBest < 0 {
		elapsedSinceBest = 0
	}

	nextScore := currentScore + 1
	denominator := math.Pow(float64(alphabetSize), -float64(nextScore)) * hashrate
	quantiles := []struct {
		label string
		q     float64
	}{
		{label: "P25", q: 0.25},
		{label: "P50", q: 0.50},
		{label: "P75", q: 0.75},
		{label: "P90", q: 0.90},
	}

	var last NextScoreRemaining
	for _, quantile := range quantiles {
		total := quantileDuration(quantile.q, denominator)
		last = NextScoreRemaining{
			Score:     nextScore,
			Quantile:  quantile.label,
			Total:     total,
			Remaining: total - elapsedSinceBest,
		}
		if elapsedSinceBest < total {
			return last, nil
		}
	}
	last.Remaining = 0
	last.Overdue = true
	return last, nil
}

func Probability(pattern vanity.Pattern) (float64, error) {
	switch pattern.Kind {
	case vanity.PatternPattern, vanity.PatternLeading:
		return math.Pow(16, -float64(pattern.Count)), nil
	case vanity.PatternTronPrefix, vanity.PatternTronSuffix:
		return math.Pow(58, -float64(pattern.Count)), nil
	default:
		return 0, fmt.Errorf("unsupported pattern kind %q", pattern.Kind)
	}
}

func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%02dh", days, hours)
}

func quantileDuration(q, denominator float64) time.Duration {
	return secondsDuration(-math.Log(1-q) / denominator)
}

func secondsDuration(seconds float64) time.Duration {
	if math.IsInf(seconds, 0) || math.IsNaN(seconds) || seconds > float64(math.MaxInt64)/float64(time.Second) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(seconds * float64(time.Second))
}
