package vanity

import (
	"fmt"
	"strconv"
	"strings"
)

type PatternKind string

const (
	PatternPattern     PatternKind = "pattern"
	PatternLeading     PatternKind = "leading"
	PatternTronPattern PatternKind = "tron-pattern"
)

type Pattern struct {
	Raw         string
	Kind        PatternKind
	Value       string
	Count       int
	Description string
}

func ParsePattern(raw string) (Pattern, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Pattern{}, fmt.Errorf("pattern is empty")
	}

	name, value, hasValue := strings.Cut(raw, ":")
	if !hasValue {
		return Pattern{}, fmt.Errorf("unsupported pattern %q", raw)
	}

	switch strings.ToLower(strings.TrimSpace(name)) {
	case "pattern":
		return parsePattern(raw, value)
	case "leading":
		return parseLeading(raw, value)
	default:
		return Pattern{}, fmt.Errorf("unsupported pattern kind %q", name)
	}
}

func ParseTronPattern(raw string) (Pattern, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Pattern{}, fmt.Errorf("pattern is empty")
	}

	name, value, hasValue := strings.Cut(raw, ":")
	if !hasValue {
		return Pattern{}, fmt.Errorf("unsupported Tron pattern %q", raw)
	}
	if strings.ToLower(strings.TrimSpace(name)) != "pattern" {
		return Pattern{}, fmt.Errorf("Tron only supports pattern:VALUE")
	}
	return parseTronPattern(raw, value)
}

func (p Pattern) MatchesAddressHex(address string) bool {
	address = normalizeAddressHex(address)
	switch p.Kind {
	case PatternPattern:
		if len(address) < len(p.Value) {
			return false
		}
		for i := 0; i < len(p.Value); i++ {
			if !isHexByte(address[i]) {
				return false
			}
			if p.Value[i] == 'X' {
				continue
			}
			if address[i] != p.Value[i] {
				return false
			}
		}
		return true
	case PatternLeading:
		if p.Value == "" {
			return false
		}
		return countLeadingByte(address, p.Value[0]) >= p.Count
	default:
		return false
	}
}

func (p Pattern) MatchesAddress(address string) bool {
	switch p.Kind {
	case PatternTronPattern:
		return p.matchesTronAddress(address)
	default:
		return p.MatchesAddressHex(address)
	}
}

func (p Pattern) ScoreAddressHex(address string) int {
	address = normalizeAddressHex(address)
	switch p.Kind {
	case PatternPattern:
		if len(address) < len(p.Value) {
			return 0
		}
		matched := 0
		for i := 0; i < len(p.Value); i++ {
			if !isHexByte(address[i]) {
				break
			}
			if p.Value[i] == 'X' {
				continue
			}
			if address[i] == p.Value[i] {
				matched++
			}
		}
		return matched
	case PatternLeading:
		if p.Value == "" {
			return 0
		}
		return countLeadingByte(address, p.Value[0])
	default:
		return 0
	}
}

func (p Pattern) ScoreAddress(address string) int {
	switch p.Kind {
	case PatternTronPattern:
		return p.scoreTronAddress(address)
	default:
		return p.ScoreAddressHex(address)
	}
}

func (p Pattern) TargetScore() int {
	switch p.Kind {
	case PatternPattern, PatternLeading, PatternTronPattern:
		return p.Count
	default:
		return 0
	}
}

func (p Pattern) String() string {
	if p.Description != "" {
		return p.Description
	}
	return p.Raw
}

func parsePattern(raw, value string) (Pattern, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Pattern{}, fmt.Errorf("pattern value is empty")
	}
	if len(value) > 40 {
		return Pattern{}, fmt.Errorf("pattern value cannot exceed 40 nibbles")
	}

	var normalized strings.Builder
	concrete := 0
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch {
		case ch >= '0' && ch <= '9':
			normalized.WriteByte(ch)
			concrete++
		case ch >= 'a' && ch <= 'f':
			normalized.WriteByte(ch)
			concrete++
		case ch >= 'A' && ch <= 'F':
			normalized.WriteByte(ch + ('a' - 'A'))
			concrete++
		case ch == 'X' || ch == 'x' || ch == '*' || ch == '?':
			normalized.WriteByte('X')
		default:
			return Pattern{}, fmt.Errorf("pattern value must contain only hex nibbles or X/x/*/? wildcards")
		}
	}

	value = normalized.String()
	return Pattern{
		Raw:         raw,
		Kind:        PatternPattern,
		Value:       value,
		Count:       concrete,
		Description: "pattern:" + value,
	}, nil
}

func parseTronPattern(raw, value string) (Pattern, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Pattern{}, fmt.Errorf("Tron pattern value is empty")
	}
	if len(value) > 34 {
		return Pattern{}, fmt.Errorf("Tron pattern value cannot exceed 34 characters")
	}
	if value[0] != 'T' {
		return Pattern{}, fmt.Errorf("Tron pattern must start with T")
	}

	var normalized strings.Builder
	concrete := 0
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch {
		case ch == '*' || ch == '?':
			if i == 0 {
				return Pattern{}, fmt.Errorf("Tron pattern must start with T")
			}
			normalized.WriteByte('?')
		case isBase58Byte(ch):
			normalized.WriteByte(ch)
			if i > 0 {
				concrete++
			}
		default:
			return Pattern{}, fmt.Errorf("Tron pattern value must contain only Base58 characters or * / ? wildcards")
		}
	}
	if concrete == 0 {
		return Pattern{}, fmt.Errorf("Tron pattern must include at least one concrete character after T")
	}

	value = normalized.String()
	return Pattern{
		Raw:         raw,
		Kind:        PatternTronPattern,
		Value:       value,
		Count:       concrete,
		Description: "pattern:" + value,
	}, nil
}

func parseLeading(raw, value string) (Pattern, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return Pattern{}, fmt.Errorf("leading pattern requires leading:H:N")
	}
	hexChar := strings.ToLower(strings.TrimSpace(parts[0]))
	if len(hexChar) != 1 || !isHexByte(hexChar[0]) {
		return Pattern{}, fmt.Errorf("leading pattern requires one hex character")
	}
	count, err := parseCount(parts[1])
	if err != nil {
		return Pattern{}, fmt.Errorf("parse leading count: %w", err)
	}
	if count > 40 {
		return Pattern{}, fmt.Errorf("leading count cannot exceed 40")
	}

	return Pattern{
		Raw:         raw,
		Kind:        PatternLeading,
		Value:       hexChar,
		Count:       count,
		Description: fmt.Sprintf("leading:%s:%d", hexChar, count),
	}, nil
}

func parseCount(value string) (int, error) {
	count, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	if count <= 0 {
		return 0, fmt.Errorf("count must be positive")
	}
	return count, nil
}

func normalizeAddressHex(address string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(address)), "0x")
}

func (p Pattern) matchesTronAddress(address string) bool {
	address = strings.TrimSpace(address)
	if len(address) < len(p.Value) {
		return false
	}
	for i := 0; i < len(p.Value); i++ {
		want := p.Value[i]
		if want == '?' {
			continue
		}
		if address[i] != want {
			return false
		}
	}
	return true
}

func (p Pattern) scoreTronAddress(address string) int {
	address = strings.TrimSpace(address)
	if len(address) < len(p.Value) {
		return 0
	}
	matched := 0
	for i := 1; i < len(p.Value); i++ {
		want := p.Value[i]
		if want == '?' {
			continue
		}
		if address[i] == want {
			matched++
		}
	}
	return matched
}

func countLeadingByte(value string, want byte) int {
	count := 0
	for i := 0; i < len(value); i++ {
		if value[i] != want {
			break
		}
		count++
	}
	return count
}

func isHexByte(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func isBase58Byte(ch byte) bool {
	switch {
	case ch >= '1' && ch <= '9':
		return true
	case ch >= 'A' && ch <= 'Z':
		return ch != 'I' && ch != 'O'
	case ch >= 'a' && ch <= 'z':
		return ch != 'l'
	default:
		return false
	}
}
