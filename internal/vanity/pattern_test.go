package vanity

import (
	"strings"
	"testing"
)

func TestParsePatternWithWildcards(t *testing.T) {
	pattern, err := ParsePattern("pattern:Deadx*?Xbeef")
	if err != nil {
		t.Fatalf("ParsePattern() error = %v", err)
	}

	if pattern.String() != "pattern:deadXXXXbeef" {
		t.Fatalf("description = %q", pattern.String())
	}
	// "deed0000beef" mismatches pattern position 2 ('e' vs 'a') so 7 of 8
	// concrete positions match; partial scores let the TUI show progress.
	if got := pattern.ScoreAddressHex("deed0000beef"); got != 7 {
		t.Fatalf("pattern partial score = %d, want 7", got)
	}
	if got := pattern.ScoreAddressHex("dead1234beef"); got != 8 {
		t.Fatalf("pattern full score = %d, want 8", got)
	}
	if pattern.TargetScore() != 8 {
		t.Fatalf("target score = %d", pattern.TargetScore())
	}
	if !pattern.MatchesAddressHex("0xdead1234beef0000000000000000000000000000") {
		t.Fatal("pattern did not match wildcard address")
	}
	if pattern.MatchesAddressHex("0xdeed1234beef0000000000000000000000000000") {
		t.Fatal("pattern matched wrong concrete nibble")
	}
}

func TestPatternAcceptsAllWildcardSpellings(t *testing.T) {
	for _, raw := range []string{"pattern:aXc", "pattern:axc", "pattern:a*c", "pattern:a?c"} {
		pattern, err := ParsePattern(raw)
		if err != nil {
			t.Fatalf("ParsePattern(%q) error = %v", raw, err)
		}
		if pattern.Value != "aXc" {
			t.Fatalf("ParsePattern(%q) value = %q", raw, pattern.Value)
		}
		if !pattern.MatchesAddressHex("abc") {
			t.Fatalf("ParsePattern(%q) did not match wildcard address", raw)
		}
		if pattern.MatchesAddressHex("agc") {
			t.Fatalf("ParsePattern(%q) matched non-hex wildcard nibble", raw)
		}
	}
}

func TestParseLeadingPattern(t *testing.T) {
	pattern, err := ParsePattern("leading:F:4")
	if err != nil {
		t.Fatalf("ParsePattern() error = %v", err)
	}

	if pattern.String() != "leading:f:4" {
		t.Fatalf("description = %q", pattern.String())
	}
	if !pattern.MatchesAddressHex("ffffabcd") {
		t.Fatal("leading pattern did not match expected address")
	}
	if pattern.MatchesAddressHex("fffabcde") {
		t.Fatal("leading pattern matched short run")
	}
	if got := pattern.ScoreAddressHex("fffffabcd"); got != 5 {
		t.Fatalf("leading score = %d", got)
	}
	if pattern.TargetScore() != 4 {
		t.Fatalf("target score = %d", pattern.TargetScore())
	}
}

func TestParseTronPattern(t *testing.T) {
	pattern, err := ParseTronPattern("pattern:TAB?*xX")
	if err != nil {
		t.Fatalf("ParseTronPattern() error = %v", err)
	}

	if pattern.Kind != PatternTronPattern {
		t.Fatalf("kind = %q", pattern.Kind)
	}
	if pattern.String() != "pattern:TAB??xX" {
		t.Fatalf("description = %q", pattern.String())
	}
	if pattern.TargetScore() != 4 {
		t.Fatalf("target score = %d", pattern.TargetScore())
	}
	if !pattern.MatchesAddress("TAB12xX000000000000000000000000000") {
		t.Fatal("Tron pattern did not match wildcard address")
	}
	if pattern.MatchesAddress("TAB12xY000000000000000000000000000") {
		t.Fatal("Tron pattern matched wrong concrete character")
	}
	if got := pattern.ScoreAddress("TAB12xY000000000000000000000000000"); got != 3 {
		t.Fatalf("Tron score = %d, want 3", got)
	}
}

func TestParseTronPatternRejectsInvalidValues(t *testing.T) {
	for _, raw := range []string{
		"leading:T:2",
		"pattern:",
		"pattern:A",
		"pattern:T",
		"pattern:T0",
		"pattern:TO",
		"pattern:Tl",
		"pattern:" + "T" + strings.Repeat("a", 34),
	} {
		if _, err := ParseTronPattern(raw); err == nil {
			t.Fatalf("ParseTronPattern(%q) succeeded, want error", raw)
		}
	}
}

func TestParsePatternRejectsOldGrammar(t *testing.T) {
	for _, raw := range []string{
		"leading-zero:4",
		"zero:4",
		"zeros:4",
		"prefix:dead",
		"matching:dead",
		"match:dead",
		"dead",
		"0xdead",
		"leading:f",
		"leading-same:6",
	} {
		if _, err := ParsePattern(raw); err == nil {
			t.Fatalf("ParsePattern(%q) succeeded, want error", raw)
		}
	}
}

func TestParsePatternRejectsInvalidNewGrammar(t *testing.T) {
	for _, raw := range []string{
		"pattern:",
		"pattern:dead_z",
		"pattern:" + strings.Repeat("a", 41),
		"leading:0:0",
		"leading:0:41",
		"leading:00:1",
		"leading:g:1",
		"leading:0",
	} {
		if _, err := ParsePattern(raw); err == nil {
			t.Fatalf("ParsePattern(%q) succeeded, want error", raw)
		}
	}
}
