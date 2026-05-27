package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

const (
	colorAccent      = "#7C5CFF"
	colorAccentHi    = "#A38CFF"
	colorPink        = "#FF5FCD"
	colorTeal        = "#5EEAD4"
	colorGreen       = "#34D399"
	colorYellow      = "#FBBF24"
	colorRed         = "#FB7185"
	colorTextStrong  = "#E2E8F0"
	colorTextMuted   = "#94A3B8"
	colorTextDim     = "#64748B"
	colorSurfaceDim  = "#1E1B2E"
	colorSurfaceLine = "#3A3357"
)

var (
	AccentStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent)).Bold(true)
	PinkStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(colorPink)).Bold(true)
	TealStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(colorTeal)).Bold(true)
	GreenStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen)).Bold(true)
	YellowStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorYellow))
	RedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed)).Bold(true)
	StrongStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextStrong))
	MutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextMuted))
	DimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim))
	HelpKeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccentHi)).Bold(true)
	HelpDescStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextMuted))
	BadgeBaseStyle = lipgloss.NewStyle().Padding(0, 1).Bold(true)
)

func Panel(title, body string, width int, accent lipgloss.Color) string {
	if accent == "" {
		accent = lipgloss.Color(colorAccent)
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 1)
	if width > 0 {
		style = style.Width(width)
	}
	if title != "" {
		header := lipgloss.NewStyle().Foreground(accent).Bold(true).Render(title)
		body = header + "\n" + body
	}
	return style.Render(body)
}

func Gradient(text, fromHex, toHex string) string {
	from, err1 := colorful.Hex(fromHex)
	to, err2 := colorful.Hex(toHex)
	if err1 != nil || err2 != nil {
		return text
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}
	var b strings.Builder
	for i, r := range runes {
		t := 0.0
		if len(runes) > 1 {
			t = float64(i) / float64(len(runes)-1)
		}
		c := from.BlendLuv(to, t).Clamped()
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex())).Bold(true)
		b.WriteString(style.Render(string(r)))
	}
	return b.String()
}

func Logo() string {
	return Gradient("PROVANITY", colorPink, colorAccent)
}

func HelpBar(items ...HelpItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		key := HelpKeyStyle.Render(item.Key)
		desc := HelpDescStyle.Render(item.Desc)
		parts = append(parts, key+" "+desc)
	}
	return DimStyle.Render(strings.Join(parts, "  ·  "))
}

type HelpItem struct {
	Key  string
	Desc string
}

func Badge(text string, fg, bg string) string {
	return BadgeBaseStyle.
		Foreground(lipgloss.Color(fg)).
		Background(lipgloss.Color(bg)).
		Render(text)
}

func AccentColor() lipgloss.Color { return lipgloss.Color(colorAccent) }
func PinkColor() lipgloss.Color   { return lipgloss.Color(colorPink) }
func GreenColor() lipgloss.Color  { return lipgloss.Color(colorGreen) }
func YellowColor() lipgloss.Color { return lipgloss.Color(colorYellow) }
func RedColor() lipgloss.Color    { return lipgloss.Color(colorRed) }
func MutedColor() lipgloss.Color  { return lipgloss.Color(colorTextMuted) }
func DimColor() lipgloss.Color    { return lipgloss.Color(colorTextDim) }
func StrongColor() lipgloss.Color { return lipgloss.Color(colorTextStrong) }
