package estimate

import (
	"math"
	"testing"
	"time"

	"github.com/woomai/provanity/internal/vanity"
)

func TestForPattern(t *testing.T) {
	pattern, err := vanity.ParsePattern("leading:0:2")
	if err != nil {
		t.Fatalf("ParsePattern: %v", err)
	}

	got, err := ForPattern(pattern, 256)
	if err != nil {
		t.Fatalf("ForPattern: %v", err)
	}
	if math.Abs(got.Probability-(1.0/256.0)) > 0.0000001 {
		t.Fatalf("probability = %f", got.Probability)
	}
	if got.Expected.Seconds() < 0.99 || got.Expected.Seconds() > 1.01 {
		t.Fatalf("expected = %s", got.Expected)
	}
}

func TestForTronPattern(t *testing.T) {
	pattern, err := vanity.ParseTronPattern("prefix:AB")
	if err != nil {
		t.Fatalf("ParseTronPattern: %v", err)
	}

	got, err := ForPattern(pattern, 58*58)
	if err != nil {
		t.Fatalf("ForPattern: %v", err)
	}
	if math.Abs(got.Probability-(1.0/(58.0*58.0))) > 0.0000001 {
		t.Fatalf("probability = %f", got.Probability)
	}
	if got.Expected.Seconds() < 0.99 || got.Expected.Seconds() > 1.01 {
		t.Fatalf("expected = %s", got.Expected)
	}
}

func TestForPatternRejectsBadHashrate(t *testing.T) {
	pattern, err := vanity.ParsePattern("pattern:dead")
	if err != nil {
		t.Fatalf("ParsePattern: %v", err)
	}
	if _, err := ForPattern(pattern, 0); err == nil {
		t.Fatal("expected bad hashrate error")
	}
}

func TestForNextScoreAdvancesQuantileAfterThreshold(t *testing.T) {
	got, err := ForNextScoreBase(1, 16, 16, 0)
	if err != nil {
		t.Fatalf("ForNextScoreBase: %v", err)
	}
	if got.Score != 2 || got.Quantile != "P25" {
		t.Fatalf("estimate = %#v", got)
	}

	got, err = ForNextScoreBase(1, 16, 16, got.Total+time.Millisecond)
	if err != nil {
		t.Fatalf("ForNextScoreBase after P25: %v", err)
	}
	if got.Quantile != "P50" {
		t.Fatalf("quantile = %s, want P50", got.Quantile)
	}
	if got.Remaining <= 0 {
		t.Fatalf("remaining should still be positive before P50: %s", got.Remaining)
	}
}

func TestForNextScoreOverdueAfterP90(t *testing.T) {
	got, err := ForNextScoreBase(1, 16, 16, 24*time.Hour)
	if err != nil {
		t.Fatalf("ForNextScoreBase: %v", err)
	}
	if got.Quantile != "P90" || !got.Overdue || got.Remaining != 0 {
		t.Fatalf("estimate = %#v", got)
	}
}
