package cli

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWizardModelGenerateDefaults(t *testing.T) {
	model := newWizardModel()
	for range 5 {
		model = wizardPress(model, tea.KeyEnter)
	}

	if !model.done {
		t.Fatal("wizard did not finish")
	}
	want := []string{
		"generate",
		"--pattern", "leading:0:10",
		"--devices", "all",
	}
	if !reflect.DeepEqual(model.args, want) {
		t.Fatalf("args = %#v, want %#v", model.args, want)
	}
}

func TestWizardHomeMenuShape(t *testing.T) {
	model := newWizardModel()

	got := wizardModeLabels(model.modes)
	want := []string{"Generate EVM Wallet", "Generate Tron Wallet", "Quit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("home labels = %#v, want %#v", got, want)
	}

	view := model.View()
	if strings.Contains(view, "Remote mode") || strings.Contains(view, "Doctor") || strings.Contains(view, "Show help") {
		t.Fatalf("home menu still includes removed entries:\n%s", view)
	}
}

func TestWizardPatternFormatShowsExamples(t *testing.T) {
	model := newWizardModel()
	model = wizardPress(model, tea.KeyEnter)

	view := model.View()
	if !strings.Contains(view, "Pattern format") {
		t.Fatalf("view did not show pattern format step:\n%s", view)
	}
	if !strings.Contains(view, "Prefix mode") || !strings.Contains(view, "Custom pattern") {
		t.Fatalf("view did not show pattern format choices:\n%s", view)
	}
}

func TestWizardGenerateSkipsPerformancePreset(t *testing.T) {
	model := newWizardModel()
	for range 4 {
		model = wizardPress(model, tea.KeyEnter)
	}

	view := model.View()
	if strings.Contains(view, "GPU performance preset") {
		t.Fatalf("view still showed performance preset step:\n%s", view)
	}
	if !strings.Contains(view, "Devices") {
		t.Fatalf("view did not advance to device selection:\n%s", view)
	}
}

func TestWizardArgsForGenerateWildcardPattern(t *testing.T) {
	args, err := wizardArgsForMode("generate", map[string]string{
		"pattern_format": "pattern",
		"pattern_value":  "DeadXXXXbeef",
	})
	if err != nil {
		t.Fatalf("wizardArgsForMode returned error: %v", err)
	}

	want := []string{
		"generate",
		"--pattern", "pattern:deadXXXXbeef",
		"--devices", "all",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestWizardArgsForGenerateTronPrefix(t *testing.T) {
	args, err := wizardArgsForMode("generate_tron", map[string]string{
		"tron_mode":         "prefix",
		"tron_prefix_value": "ABC",
	})
	if err != nil {
		t.Fatalf("wizardArgsForMode returned error: %v", err)
	}

	want := []string{
		"generate-tron",
		"--pattern", "prefix:ABC",
		"--devices", "all",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestWizardArgsForGenerateTronSuffix(t *testing.T) {
	args, err := wizardArgsForMode("generate_tron", map[string]string{
		"tron_mode":         "suffix",
		"tron_suffix_value": "xyz",
	})
	if err != nil {
		t.Fatalf("wizardArgsForMode returned error: %v", err)
	}

	want := []string{
		"generate-tron",
		"--pattern", "suffix:xyz",
		"--devices", "all",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestWizardArgsForGenerateIgnoresAdvancedOutputOptions(t *testing.T) {
	args, err := wizardArgsForMode("generate", map[string]string{
		"dashboard": "no",
		"output":    "result.json",
	})
	if err != nil {
		t.Fatalf("wizardArgsForMode returned error: %v", err)
	}

	for _, arg := range args {
		if arg == "--no-tui" || arg == "--output" {
			t.Fatalf("wizard args unexpectedly included advanced output option: %#v", args)
		}
	}
}

func TestWizardModelQuit(t *testing.T) {
	model := newWizardModel()
	model = wizardPress(model, tea.KeyDown)
	model = wizardPress(model, tea.KeyDown)
	model = wizardPress(model, tea.KeyEnter)
	if !model.canceled {
		t.Fatal("wizard was not canceled")
	}
}

func TestWizardArgsForBenchCustomDevices(t *testing.T) {
	args, err := wizardArgsForMode("bench", map[string]string{
		"devices":         "custom",
		"device_ids":      "0,1",
		"duration":        "custom",
		"duration_custom": "45s",
		"progress":        "2000",
		"json":            "yes",
	})
	if err != nil {
		t.Fatalf("wizardArgsForMode returned error: %v", err)
	}

	want := []string{
		"bench",
		"--devices", "0,1",
		"--duration", "45s",
		"--progress-interval", "1000",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func wizardPress(model wizardModel, key tea.KeyType) wizardModel {
	updated, _ := model.Update(tea.KeyMsg{Type: key})
	return updated.(wizardModel)
}

func wizardType(model wizardModel, s string) wizardModel {
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return updated.(wizardModel)
}

func TestWizardTronModeViewShowsLiveExampleAndValidation(t *testing.T) {
	model := newWizardModel()
	model = wizardPress(model, tea.KeyDown)  // home: Generate EVM -> Generate Tron
	model = wizardPress(model, tea.KeyEnter) // select Tron -> land on tron_mode field

	if view := model.View(); !strings.Contains(view, "prefix:ABC") {
		t.Fatalf("tron_mode view missing live prefix example:\n%s", view)
	}
	model = wizardPress(model, tea.KeyDown) // highlight suffix
	if view := model.View(); !strings.Contains(view, "suffix:xyz") {
		t.Fatalf("tron_mode view did not update example to suffix:\n%s", view)
	}

	model = wizardPress(model, tea.KeyUp)    // back to prefix
	model = wizardPress(model, tea.KeyEnter) // select prefix -> tron_prefix_value field
	model = wizardType(model, "0")           // '0' is not a Base58 character
	if view := model.View(); !strings.Contains(view, "invalid") {
		t.Fatalf("typing an invalid character did not surface a live error:\n%s", view)
	}
}

func wizardModeLabels(modes []wizardMode) []string {
	labels := make([]string, 0, len(modes))
	for _, mode := range modes {
		labels = append(labels, mode.label)
	}
	return labels
}

func tronFieldByKey(t *testing.T, key string) wizardField {
	t.Helper()
	for _, f := range tronPatternFields() {
		if f.key == key {
			return f
		}
	}
	t.Fatalf("tron field %q not found", key)
	return wizardField{}
}

func TestWizardTronModePreviewTracksChoice(t *testing.T) {
	modeField := tronFieldByKey(t, "tron_mode")
	if got := (wizardModel{choiceCursor: 0}).fieldPreview(modeField); !strings.Contains(got, "prefix:ABC") {
		t.Fatalf("prefix preview = %q, want it to mention prefix:ABC", got)
	}
	if got := (wizardModel{choiceCursor: 1}).fieldPreview(modeField); !strings.Contains(got, "suffix:xyz") {
		t.Fatalf("suffix preview = %q, want it to mention suffix:xyz", got)
	}
}

func TestWizardLiveStatusValidatesTronInput(t *testing.T) {
	prefixField := tronFieldByKey(t, "tron_prefix_value")
	var m wizardModel
	if s := m.liveStatus(prefixField, ""); s != "" {
		t.Fatalf("empty input status = %q, want empty", s)
	}
	if s := m.liveStatus(prefixField, "AB"); s != "" {
		t.Fatalf("valid input status = %q, want empty", s)
	}
	if s := m.liveStatus(prefixField, "0"); s == "" {
		t.Fatal("invalid base58 input should produce a status")
	}
	if s := m.liveStatus(prefixField, "z"); s == "" {
		t.Fatal("unreachable prefix should produce a status")
	}

	suffixField := tronFieldByKey(t, "tron_suffix_value")
	if s := m.liveStatus(suffixField, "abcdefghi"); s == "" {
		t.Fatal("over-long suffix should produce a status")
	}
}
