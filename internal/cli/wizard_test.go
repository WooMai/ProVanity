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

func TestWizardArgsForGenerateTronPattern(t *testing.T) {
	args, err := wizardArgsForMode("generate_tron", map[string]string{
		"tron_pattern_value": "TA?X",
	})
	if err != nil {
		t.Fatalf("wizardArgsForMode returned error: %v", err)
	}

	want := []string{
		"generate-tron",
		"--pattern", "pattern:TA?X",
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

func wizardModeLabels(modes []wizardMode) []string {
	labels := make([]string, 0, len(modes))
	for _, mode := range modes {
		labels = append(labels, mode.label)
	}
	return labels
}
