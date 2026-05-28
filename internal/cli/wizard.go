package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/woomai/provanity/internal/local"
	"github.com/woomai/provanity/internal/tui"
	"github.com/woomai/provanity/internal/vanity"
)

type devicesProbedMsg struct {
	count int
}

type wizardMode struct {
	key         string
	label       string
	description string
}

type wizardChoice struct {
	key         string
	label       string
	description string
}

type wizardFieldKind int

const (
	wizardFieldText wizardFieldKind = iota
	wizardFieldChoice
)

type wizardField struct {
	key          string
	title        string
	description  string
	kind         wizardFieldKind
	defaultValue string
	required     bool
	choices      []wizardChoice
	include      func(map[string]string) bool
	validate     func(string) error
	placeholder  bool
	singleChar   bool
}

type wizardModel struct {
	modes        []wizardMode
	modeCursor   int
	selectedMode string
	fields       []wizardField
	fieldIndex   int
	choiceCursor int
	input        string
	values       map[string]string
	width        int
	canceled     bool
	done         bool
	args         []string
	err          error
	status       string
}

var (
	wizardSubtleStyle     = tui.MutedStyle
	wizardSelectedStyle   = tui.TealStyle
	wizardErrorStyle      = tui.RedStyle
	wizardInfoStyle       = lipgloss.NewStyle().Foreground(tui.YellowColor())
	wizardCursorStyle     = lipgloss.NewStyle().Foreground(tui.PinkColor()).Bold(true)
	wizardInputStyle      = lipgloss.NewStyle().Foreground(tui.StrongColor()).Bold(true)
	wizardChoiceLabelDim  = lipgloss.NewStyle().Foreground(tui.MutedColor())
	wizardChoiceDescStyle = lipgloss.NewStyle().Foreground(tui.DimColor())
)

func runInteractiveWizard(cmd *cobra.Command) error {
	if !isTerminal() {
		return cmd.Help()
	}

	model := newWizardModel()
	program := tea.NewProgram(
		model,
		tea.WithInput(cmd.InOrStdin()),
		tea.WithOutput(cmd.OutOrStdout()),
		tea.WithAltScreen(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return err
	}

	final := finalModel.(wizardModel)
	if final.canceled {
		fmt.Fprintln(cmd.OutOrStdout(), "wizard canceled")
		return nil
	}
	if final.err != nil {
		return final.err
	}
	if len(final.args) == 0 {
		return cmd.Help()
	}

	next := NewRootCommand()
	next.SetArgs(final.args)
	next.SetIn(cmd.InOrStdin())
	next.SetOut(cmd.OutOrStdout())
	next.SetErr(cmd.ErrOrStderr())
	return next.ExecuteContext(cmd.Context())
}

func newWizardModel() wizardModel {
	return wizardModel{
		modes: []wizardMode{
			{key: "generate", label: "Generate EVM Wallet", description: "Search for an EVM address and print the final private key."},
			{key: "generate_tron", label: "Generate Tron Wallet", description: "Search for a Tron address and print the final private key."},
			{key: "quit", label: "Quit", description: "Leave without running a command."},
		},
		values: make(map[string]string),
		status: "",
	}
}

func (m wizardModel) Init() tea.Cmd {
	return probeDevicesCmd()
}

func probeDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		devs, err := local.ProbeDevices(ctx)
		if err != nil {
			return devicesProbedMsg{count: -1}
		}
		return devicesProbedMsg{count: len(devs)}
	}
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case devicesProbedMsg:
		if msg.count == 1 {
			m.values["devices"] = "all"
			m.values["_device_count"] = "1"
			if m.selectedMode != "" && m.fieldIndex >= 0 && m.fieldIndex < len(m.fields) {
				if m.fields[m.fieldIndex].key == "devices" {
					if m.nextField() {
						m.resetCurrentField()
					} else {
						m.finish()
						return m, tea.Quit
					}
				}
			}
		}
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m wizardModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.canceled = true
		return m, tea.Quit
	case "left":
		if m.selectedMode != "" {
			m.previousField()
		}
		return m, nil
	}

	if m.selectedMode == "" {
		return m.updateModeKey(msg)
	}
	return m.updateFieldKey(msg)
}

func (m wizardModel) updateModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	modes := m.currentModes()
	if len(modes) == 0 {
		return m, nil
	}
	if m.modeCursor >= len(modes) {
		m.modeCursor = len(modes) - 1
	}

	switch msg.String() {
	case "up":
		if m.modeCursor > 0 {
			m.modeCursor--
		}
	case "down":
		if m.modeCursor < len(modes)-1 {
			m.modeCursor++
		}
	case "enter":
		mode := modes[m.modeCursor].key
		switch mode {
		case "quit":
			m.canceled = true
			return m, tea.Quit
		default:
			m.selectedMode = mode
			m.fields = wizardFields(mode)
			m.fieldIndex = m.firstVisibleField()
			m.resetCurrentField()
		}
	}
	return m, nil
}

func (m wizardModel) updateFieldKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	field, ok := m.currentField()
	if !ok {
		m.finish()
		return m, tea.Quit
	}

	if field.kind == wizardFieldChoice {
		switch msg.String() {
		case "up":
			if m.choiceCursor > 0 {
				m.choiceCursor--
			}
		case "down":
			if m.choiceCursor < len(field.choices)-1 {
				m.choiceCursor++
			}
		case "enter":
			m.values[field.key] = field.choices[m.choiceCursor].key
			if m.nextField() {
				m.resetCurrentField()
				return m, nil
			}
			m.finish()
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "enter":
		value := strings.TrimSpace(m.input)
		if value == "" && field.placeholder {
			value = field.defaultValue
		}
		if err := validateWizardField(field, value); err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.values[field.key] = value
		if m.nextField() {
			m.resetCurrentField()
			return m, nil
		}
		m.finish()
		return m, tea.Quit
	case "backspace":
		m.input = trimLastRune(m.input)
	case " ":
		if !field.singleChar {
			m.input += " "
		}
	default:
		if msg.Type == tea.KeyRunes {
			if field.singleChar {
				runes := msg.Runes
				if len(runes) > 0 {
					m.input = string(runes[len(runes)-1])
				}
			} else {
				m.input += string(msg.Runes)
			}
		}
	}
	// Validate as the user types so an invalid format is flagged immediately;
	// Enter stays blocked until it parses (see the "enter" case above).
	m.status = m.liveStatus(field, m.input)
	return m, nil
}

// liveStatus returns the validation error for the in-progress text input, or ""
// when the value is empty (nothing to flag yet) or valid. It reuses the same
// check Enter runs, so the live message matches the one that blocks advancing.
func (m wizardModel) liveStatus(field wizardField, input string) string {
	if field.kind != wizardFieldText {
		return ""
	}
	value := strings.TrimSpace(input)
	if value == "" {
		return ""
	}
	if err := validateWizardField(field, value); err != nil {
		return err.Error()
	}
	return ""
}

func (m *wizardModel) finish() {
	args, err := wizardArgsForMode(m.selectedMode, m.values)
	if err != nil {
		m.err = err
		return
	}
	m.args = args
	m.done = true
}

func (m wizardModel) View() string {
	width := m.width
	if width <= 0 {
		width = 88
	}
	if width > 110 {
		width = 110
	}
	panelWidth := width - 4
	if panelWidth < 50 {
		panelWidth = 50
	}

	var b strings.Builder
	b.WriteString(wizardHeader(panelWidth))
	b.WriteString("\n\n")

	if m.selectedMode == "" {
		b.WriteString(m.modeView(panelWidth))
	} else {
		b.WriteString(m.fieldView(panelWidth))
	}

	b.WriteString("\n")
	if m.status != "" {
		style := wizardSubtleStyle
		if strings.HasPrefix(m.status, "invalid") || strings.Contains(m.status, "required") || strings.Contains(m.status, "must") {
			style = wizardErrorStyle
		} else if strings.HasPrefix(m.status, "Enter") {
			style = wizardInfoStyle
		}
		b.WriteString("  ")
		b.WriteString(style.Render(m.status))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(wizardFooter(m))
	b.WriteString("\n")
	return b.String()
}

func wizardHeader(width int) string {
	logo := tui.Logo()
	return tui.Panel("", logo, width, tui.AccentColor())
}

func wizardFooter(m wizardModel) string {
	items := []tui.HelpItem{
		{Key: "↑↓", Desc: "move"},
		{Key: "Enter", Desc: "confirm"},
	}
	if m.selectedMode != "" {
		items = append(items, tui.HelpItem{Key: "←", Desc: "back"})
	}
	items = append(items, tui.HelpItem{Key: "esc", Desc: "cancel"})
	return tui.HelpBar(items...)
}

func (m wizardModel) modeView(width int) string {
	modes := m.currentModes()
	labelWidth := 0
	for _, mode := range modes {
		if w := len([]rune(mode.label)); w > labelWidth {
			labelWidth = w
		}
	}
	labelWidth += 2

	var inner strings.Builder
	for i, mode := range modes {
		cursor := "  "
		labelStyle := wizardChoiceLabelDim
		descStyle := wizardChoiceDescStyle
		if i == m.modeCursor {
			cursor = wizardCursorStyle.Render("▸ ")
			labelStyle = wizardSelectedStyle
			descStyle = wizardSubtleStyle
		}
		inner.WriteString(cursor)
		inner.WriteString(labelStyle.Render(padRight(mode.label, labelWidth)))
		if mode.description != "" {
			inner.WriteString(descStyle.Render(mode.description))
		}
		if i != len(modes)-1 {
			inner.WriteString("\n")
		}
	}
	return tui.Panel("Pick a mode", inner.String(), width, tui.AccentColor())
}

func (m wizardModel) currentModes() []wizardMode {
	return m.modes
}

func (m wizardModel) fieldView(width int) string {
	field, ok := m.currentField()
	if !ok {
		return ""
	}

	visibleIndex, visibleTotal := m.visiblePosition()
	stepBadge := tui.Badge(fmt.Sprintf(" %s ", strings.ToUpper(wizardModeLabel(m.selectedMode))), "#0B0B14", "#7C5CFF")
	stepLabel := wizardSubtleStyle.Render(fmt.Sprintf("step %d / %d", visibleIndex, visibleTotal))
	heading := stepBadge + "  " + stepLabel

	var inner strings.Builder
	inner.WriteString(heading)
	inner.WriteString("\n")
	if field.description != "" {
		inner.WriteString(wizardSubtleStyle.Render(truncateWizard(field.description, width-4)))
		inner.WriteString("\n")
	}
	inner.WriteString("\n")

	if field.kind == wizardFieldChoice {
		for i, choice := range field.choices {
			cursor := "  "
			labelStyle := wizardChoiceLabelDim
			descStyle := wizardChoiceDescStyle
			if i == m.choiceCursor {
				cursor = wizardCursorStyle.Render("▸ ")
				labelStyle = wizardSelectedStyle
				descStyle = wizardSubtleStyle
			}
			inner.WriteString(cursor)
			inner.WriteString(labelStyle.Render(choice.label))
			if choice.description != "" {
				inner.WriteString("\n    ")
				inner.WriteString(descStyle.Render(choice.description))
			}
			if i != len(field.choices)-1 {
				inner.WriteString("\n")
			}
		}
	} else {
		value := m.input
		inner.WriteString(wizardCursorStyle.Render("❯ "))
		if value == "" {
			hint := "(start typing)"
			if field.placeholder && field.defaultValue != "" {
				hint = field.defaultValue
			}
			inner.WriteString(wizardChoiceDescStyle.Render(hint))
		} else {
			inner.WriteString(wizardInputStyle.Render(truncateWizard(value, width-6)))
		}
		inner.WriteString(wizardCursorStyle.Render("_"))
	}

	if preview := m.fieldPreview(field); preview != "" {
		inner.WriteString("\n\n")
		inner.WriteString(wizardChoiceDescStyle.Render(truncateWizard(preview, width-4)))
	}

	panel := tui.Panel(field.title, inner.String(), width, tui.PinkColor())

	if summary := m.summaryView(width); summary != "" {
		return panel + "\n\n" + summary
	}
	return panel
}

func (m wizardModel) fieldPreview(field wizardField) string {
	switch field.key {
	case "pattern_value":
		value := strings.TrimSpace(m.input)
		if value == "" {
			value = field.defaultValue
		}
		if pattern, err := vanity.ParsePattern("pattern:" + value); err == nil {
			value = strings.TrimPrefix(pattern.String(), "pattern:")
		}
		return fmt.Sprintf("Result example: --pattern pattern:%s -> 0x%s...", value, value)
	case "pattern_leading_hex":
		leading := strings.ToLower(strings.TrimSpace(m.input))
		if leading == "" {
			leading = field.defaultValue
		}
		count := valueOrDefault(m.values, "pattern_leading_count", "10")
		return fmt.Sprintf("Result example: --pattern leading:%s:%s -> 0x%s...", leading, count, repeatedLeadingPreview(leading, count))
	case "pattern_leading_count":
		count := strings.TrimSpace(m.input)
		if count == "" {
			count = field.defaultValue
		}
		leading := valueOrDefault(m.values, "pattern_leading_hex", "0")
		return fmt.Sprintf("Result example: --pattern leading:%s:%s -> 0x%s...", leading, count, repeatedLeadingPreview(leading, count))
	case "tron_prefix_value":
		value := strings.TrimSpace(m.input)
		if value == "" {
			value = field.defaultValue
		}
		if pattern, err := vanity.ParseTronPattern("prefix:" + value); err == nil {
			value = strings.TrimPrefix(pattern.String(), "prefix:")
		}
		return fmt.Sprintf("Result example: --pattern prefix:%s -> T%s...", value, value)
	case "tron_suffix_value":
		value := strings.TrimSpace(m.input)
		if value == "" {
			value = field.defaultValue
		}
		if pattern, err := vanity.ParseTronPattern("suffix:" + value); err == nil {
			value = strings.TrimPrefix(pattern.String(), "suffix:")
		}
		return fmt.Sprintf("Result example: --pattern suffix:%s -> ...%s", value, value)
	case "tron_mode":
		// Live example for the highlighted choice so the difference between
		// prefix and suffix is obvious before picking one.
		mode := "prefix"
		if m.choiceCursor >= 0 && m.choiceCursor < len(field.choices) {
			mode = field.choices[m.choiceCursor].key
		}
		if mode == "suffix" {
			return "Result example: --pattern suffix:xyz -> address ends ...xyz"
		}
		return "Result example: --pattern prefix:ABC -> address starts TABC..."
	default:
		return ""
	}
}

func (m wizardModel) summaryView(width int) string {
	var rows []string
	for _, field := range m.fields {
		if !m.fieldVisible(field) {
			continue
		}
		value, ok := m.values[field.key]
		if !ok || value == "" {
			continue
		}
		key := wizardChoiceLabelDim.Render(padRight(field.title+":", 22))
		val := wizardInputStyle.Render(displayWizardValue(field, value))
		rows = append(rows, key+" "+val)
	}
	if len(rows) == 0 {
		return ""
	}
	body := strings.Join(rows, "\n")
	return tui.Panel("Current selections", body, width, tui.MutedColor())
}

func padRight(value string, width int) string {
	runes := []rune(value)
	if len(runes) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(runes))
}

func (m wizardModel) currentField() (wizardField, bool) {
	if m.fieldIndex < 0 || m.fieldIndex >= len(m.fields) {
		return wizardField{}, false
	}
	field := m.fields[m.fieldIndex]
	if !m.fieldVisible(field) {
		return wizardField{}, false
	}
	return field, true
}

func (m wizardModel) fieldVisible(field wizardField) bool {
	return field.include == nil || field.include(m.values)
}

func (m wizardModel) firstVisibleField() int {
	for i, field := range m.fields {
		if m.fieldVisible(field) {
			return i
		}
	}
	return -1
}

func (m *wizardModel) nextField() bool {
	for i := m.fieldIndex + 1; i < len(m.fields); i++ {
		if m.fieldVisible(m.fields[i]) {
			m.fieldIndex = i
			return true
		}
	}
	return false
}

func (m *wizardModel) previousField() bool {
	for i := m.fieldIndex - 1; i >= 0; i-- {
		if m.fieldVisible(m.fields[i]) {
			m.fieldIndex = i
			m.resetCurrentField()
			return true
		}
	}
	mode := m.selectedMode
	m.selectedMode = ""
	m.fields = nil
	m.fieldIndex = 0
	m.modeCursor = modeIndex(m.currentModes(), mode)
	m.status = ""
	return true
}

func (m *wizardModel) resetCurrentField() {
	field, ok := m.currentField()
	if !ok {
		return
	}
	m.status = ""
	value := valueOrDefault(m.values, field.key, field.defaultValue)
	if field.kind == wizardFieldText {
		if field.placeholder {
			if existing, ok := m.values[field.key]; ok {
				m.input = existing
			} else {
				m.input = ""
			}
		} else {
			m.input = value
		}
		return
	}
	m.input = ""
	m.choiceCursor = 0
	for i, choice := range field.choices {
		if choice.key == value {
			m.choiceCursor = i
			return
		}
	}
}

func (m wizardModel) visiblePosition() (int, int) {
	position := 0
	total := 0
	for i, field := range m.fields {
		if !m.fieldVisible(field) {
			continue
		}
		total++
		if i <= m.fieldIndex {
			position = total
		}
	}
	return position, total
}

func wizardFields(mode string) []wizardField {
	switch mode {
	case "generate":
		return append(patternFields(),
			devicesField(true),
			deviceIDsField(),
		)
	case "generate_tron":
		return append(tronPatternFields(),
			devicesField(true),
			deviceIDsField(),
		)
	case "bench":
		return []wizardField{
			devicesField(false),
			deviceIDsField(),
			durationField("duration", "Duration", "10s"),
			customDurationField("duration_custom", "Custom duration", "duration"),
		}
	default:
		return nil
	}
}

func tronPatternFields() []wizardField {
	return []wizardField{
		choiceFieldWithDescription("tron_mode", "Tron match mode", "prefix", "Match the start or the end of the address.", []wizardChoice{
			{key: "prefix", label: "Prefix mode", description: "Characters right after the leading T — e.g. prefix:ABC -> TABC…"},
			{key: "suffix", label: "Suffix mode", description: "Trailing characters of the address — e.g. suffix:xyz -> …xyz"},
		}),
		placeholderTextField("tron_prefix_value", "Prefix value", "ABC", validateTronPrefixValue, valueIs("tron_mode", "prefix"), fmt.Sprintf("1–%d Base58 characters after the implicit leading T.", vanity.MaxTronConcretePos), false),
		placeholderTextField("tron_suffix_value", "Suffix value", "xyz", validateTronSuffixValue, valueIs("tron_mode", "suffix"), fmt.Sprintf("1–%d trailing Base58 characters.", vanity.MaxTronSuffixLen), false),
	}
}

func patternFields() []wizardField {
	return []wizardField{
		choiceFieldWithDescription("pattern_format", "Pattern format", "leading", "Choose the pattern shape first; the next steps ask only for its parameters.", []wizardChoice{
			{key: "leading", label: "Prefix mode", description: "Repeat one hex character at the start of the address — e.g. 0x0000…"},
			{key: "pattern", label: "Custom pattern", description: "Exact nibbles with X / * wildcards — e.g. deadXXXXbeef."},
		}),
		placeholderTextField("pattern_leading_hex", "Prefix character", "0", validateLeadingHex, valueIs("pattern_format", "leading"), "One hex digit (0–f). Typing replaces the current value.", true),
		placeholderTextField("pattern_leading_count", "Prefix length", "10", validateLeadingCount, valueIs("pattern_format", "leading"), "How many leading characters to match (1–40). Press Enter to use the placeholder.", false),
		placeholderTextField("pattern_value", "Pattern value", "deadXXXXbeef", validatePatternValue, valueIs("pattern_format", "pattern"), "1–40 hex nibbles; X, x, *, ? each match one nibble. Press Enter to use the placeholder.", false),
	}
}

func devicesField(allowSelect bool) wizardField {
	choices := []wizardChoice{
		{key: "all", label: "All devices", description: "Use every CUDA device reported by the backend."},
	}
	if allowSelect {
		choices = append(choices, wizardChoice{key: "select", label: "Select GPUs before running", description: "Probe devices and open the GPU picker."})
	}
	choices = append(choices, wizardChoice{key: "custom", label: "Custom device ids", description: "Enter a comma-separated list such as 0,1."})
	field := choiceField("devices", "Devices", "all", choices)
	field.include = func(values map[string]string) bool {
		return values["_device_count"] != "1"
	}
	return field
}

func deviceIDsField() wizardField {
	return textFieldWithInclude("device_ids", "Device ids", "0", true, func(value string) error {
		_, requestedSelection, err := local.ParseDeviceIDs(value)
		if err != nil {
			return err
		}
		if requestedSelection {
			return fmt.Errorf("use the Select GPUs option instead")
		}
		return nil
	}, valueIs("devices", "custom"))
}

func durationField(key, title, defaultValue string) wizardField {
	return durationFieldWithInclude(key, title, defaultValue, nil)
}

func durationFieldWithInclude(key, title, defaultValue string, include func(map[string]string) bool) wizardField {
	return choiceFieldWithInclude(key, title, defaultValue, []wizardChoice{
		{key: "2s", label: "2s", description: "Quick health check."},
		{key: "10s", label: "10s", description: "Short benchmark."},
		{key: "30s", label: "30s", description: "More stable sample."},
		{key: "60s", label: "60s", description: "Release-style baseline."},
		{key: "custom", label: "Custom", description: "Enter another Go duration such as 90s or 2m."},
	}, include)
}

func textFieldWithInclude(key, title, defaultValue string, required bool, validate func(string) error, include func(map[string]string) bool) wizardField {
	return wizardField{
		key:          key,
		title:        title,
		kind:         wizardFieldText,
		defaultValue: defaultValue,
		required:     required,
		validate:     validate,
		include:      include,
	}
}

func placeholderTextField(key, title, defaultValue string, validate func(string) error, include func(map[string]string) bool, description string, singleChar bool) wizardField {
	field := textFieldWithInclude(key, title, defaultValue, true, validate, include)
	field.description = description
	field.placeholder = true
	field.singleChar = singleChar
	return field
}

func choiceField(key, title, defaultValue string, choices []wizardChoice) wizardField {
	return choiceFieldWithInclude(key, title, defaultValue, choices, nil)
}

func choiceFieldWithDescription(key, title, defaultValue, description string, choices []wizardChoice) wizardField {
	field := choiceFieldWithInclude(key, title, defaultValue, choices, nil)
	field.description = description
	return field
}

func choiceFieldWithInclude(key, title, defaultValue string, choices []wizardChoice, include func(map[string]string) bool) wizardField {
	return wizardField{
		key:          key,
		title:        title,
		kind:         wizardFieldChoice,
		defaultValue: defaultValue,
		required:     true,
		choices:      choices,
		include:      include,
	}
}

func validatePatternValue(value string) error {
	_, err := vanity.ParsePattern("pattern:" + strings.TrimSpace(value))
	return err
}

func validateTronPrefixValue(value string) error {
	_, err := vanity.ParseTronPattern("prefix:" + strings.TrimSpace(value))
	return err
}

func validateTronSuffixValue(value string) error {
	_, err := vanity.ParseTronPattern("suffix:" + strings.TrimSpace(value))
	return err
}

func validateLeadingHex(value string) error {
	_, err := vanity.ParsePattern("leading:" + strings.TrimSpace(value) + ":1")
	return err
}

func validateLeadingCount(value string) error {
	_, err := vanity.ParsePattern("leading:0:" + strings.TrimSpace(value))
	return err
}

func customDurationField(key, title, parentKey string) wizardField {
	return textFieldWithInclude(key, title, "", true, validateDuration, valueIs(parentKey, "custom"))
}

func validateWizardField(field wizardField, value string) error {
	if field.required && value == "" {
		return fmt.Errorf("%s is required", field.title)
	}
	if value != "" && field.validate != nil {
		if err := field.validate(value); err != nil {
			return fmt.Errorf("invalid %s: %w", strings.ToLower(field.title), err)
		}
	}
	return nil
}

func validateDuration(value string) error {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	if duration <= 0 {
		return fmt.Errorf("must be positive")
	}
	return nil
}

func valueIs(key, want string) func(map[string]string) bool {
	return func(values map[string]string) bool {
		return values[key] == want
	}
}

func wizardArgsForMode(mode string, values map[string]string) ([]string, error) {
	switch mode {
	case "generate":
		pattern, err := resolveWizardPattern(values)
		if err != nil {
			return nil, err
		}
		args := []string{"generate", "--pattern", pattern}
		args = appendDeviceArgs(args, values, true)
		return args, nil
	case "generate_tron":
		pattern, err := resolveWizardTronPattern(values)
		if err != nil {
			return nil, err
		}
		args := []string{"generate-tron", "--pattern", pattern}
		args = appendDeviceArgs(args, values, true)
		return args, nil
	case "bench":
		args := []string{"bench"}
		args = appendDeviceArgs(args, values, false)
		args = append(args,
			"--duration", resolveCustom(values, "duration", "duration_custom", "10s"),
			"--progress-interval", "1000",
		)
		return args, nil
	default:
		return nil, fmt.Errorf("unsupported wizard mode %q", mode)
	}
}

func resolveWizardTronPattern(values map[string]string) (string, error) {
	if raw := strings.TrimSpace(values["pattern"]); raw != "" {
		pattern, err := vanity.ParseTronPattern(raw)
		if err != nil {
			return "", err
		}
		return pattern.String(), nil
	}

	var raw string
	switch valueOrDefault(values, "tron_mode", "prefix") {
	case "suffix":
		raw = "suffix:" + strings.TrimSpace(valueOrDefault(values, "tron_suffix_value", "xyz"))
	default:
		raw = "prefix:" + strings.TrimSpace(valueOrDefault(values, "tron_prefix_value", "ABC"))
	}
	pattern, err := vanity.ParseTronPattern(raw)
	if err != nil {
		return "", err
	}
	return pattern.String(), nil
}

func resolveWizardPattern(values map[string]string) (string, error) {
	if raw := strings.TrimSpace(values["pattern"]); raw != "" {
		pattern, err := vanity.ParsePattern(raw)
		if err != nil {
			return "", err
		}
		return pattern.String(), nil
	}

	var raw string
	switch valueOrDefault(values, "pattern_format", "leading") {
	case "pattern":
		raw = "pattern:" + strings.TrimSpace(valueOrDefault(values, "pattern_value", "deadXXXXbeef"))
	case "leading":
		raw = "leading:" +
			strings.TrimSpace(valueOrDefault(values, "pattern_leading_hex", "0")) +
			":" +
			strings.TrimSpace(valueOrDefault(values, "pattern_leading_count", "10"))
	default:
		return "", fmt.Errorf("unsupported pattern format %q", values["pattern_format"])
	}

	pattern, err := vanity.ParsePattern(raw)
	if err != nil {
		return "", err
	}
	return pattern.String(), nil
}

func appendDeviceArgs(args []string, values map[string]string, allowSelect bool) []string {
	switch valueOrDefault(values, "devices", "all") {
	case "select":
		if allowSelect {
			return append(args, "--select-gpu")
		}
		return append(args, "--devices", "all")
	case "custom":
		return append(args, "--devices", strings.TrimSpace(values["device_ids"]))
	default:
		return append(args, "--devices", "all")
	}
}

func resolveCustom(values map[string]string, key, customKey, defaultValue string) string {
	value := valueOrDefault(values, key, defaultValue)
	if value == "custom" {
		return strings.TrimSpace(values[customKey])
	}
	return value
}

func valueOrDefault(values map[string]string, key, defaultValue string) string {
	if value, ok := values[key]; ok {
		return value
	}
	return defaultValue
}

func displayWizardValue(field wizardField, value string) string {
	if field.kind == wizardFieldChoice {
		for _, choice := range field.choices {
			if choice.key == value {
				return choice.label
			}
		}
	}
	return value
}

func repeatedLeadingPreview(leading, countRaw string) string {
	count, err := strconv.Atoi(strings.TrimSpace(countRaw))
	if err != nil || count <= 0 {
		count = 1
	}
	if count > 8 {
		count = 8
	}
	if leading == "" {
		leading = "0"
	}
	return strings.Repeat(leading[:1], count)
}

func wizardModeLabel(key string) string {
	model := newWizardModel()
	for _, mode := range model.modes {
		if mode.key == key {
			return mode.label
		}
	}
	return key
}

func modeIndex(modes []wizardMode, key string) int {
	for i, mode := range modes {
		if mode.key == key {
			return i
		}
	}
	return 0
}

func trimLastRune(value string) string {
	if value == "" {
		return ""
	}
	runes := []rune(value)
	return string(runes[:len(runes)-1])
}

func truncateWizard(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 3 {
		return value[:width]
	}
	return value[:width-3] + "..."
}
