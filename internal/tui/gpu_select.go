package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/woomai/provanity/internal/gpu"
)

type gpuSelectModel struct {
	devices  []gpu.Device
	cursor   int
	selected map[int]bool
	width    int
	status   string
	canceled bool
}

func RunGPUSelect(devices []gpu.Device) ([]int, error) {
	if len(devices) == 0 {
		return nil, fmt.Errorf("no GPU devices reported")
	}

	selected := make(map[int]bool, len(devices))
	for _, device := range devices {
		selected[device.ID] = true
	}
	model := gpuSelectModel{
		devices:  devices,
		selected: selected,
		status:   "space toggles · enter starts",
	}

	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return nil, err
	}
	final := finalModel.(gpuSelectModel)
	if final.canceled {
		return nil, fmt.Errorf("GPU selection canceled")
	}

	ids := make([]int, 0, len(final.devices))
	for _, device := range final.devices {
		if final.selected[device.ID] {
			ids = append(ids, device.ID)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no GPU devices selected")
	}
	return ids, nil
}

func (m gpuSelectModel) Init() tea.Cmd {
	return nil
}

func (m gpuSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.canceled = true
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.devices)-1 {
				m.cursor++
			}
		case " ":
			id := m.devices[m.cursor].ID
			m.selected[id] = !m.selected[id]
		case "a":
			selectAll := m.countSelected() != len(m.devices)
			for _, device := range m.devices {
				m.selected[device.ID] = selectAll
			}
		case "enter":
			if m.countSelected() == 0 {
				m.status = "select at least one GPU"
				return m, nil
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m gpuSelectModel) View() string {
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
	b.WriteString(Panel("", Logo(), panelWidth, AccentColor()))
	b.WriteString("\n\n")

	var rows []string
	for i, device := range m.devices {
		cursor := "  "
		check := DimStyle.Render("[ ]")
		labelStyle := MutedStyle
		nameStyle := DimStyle
		if m.selected[device.ID] {
			check = GreenStyle.Render("[✓]")
			labelStyle = AccentStyle
			nameStyle = StrongStyle
		}
		if i == m.cursor {
			cursor = PinkStyle.Render("▸ ")
			labelStyle = TealStyle
			nameStyle = StrongStyle.Bold(true)
		}
		row := cursor + check + " " + labelStyle.Render(fmt.Sprintf("GPU%d", device.ID)) + "  " + nameStyle.Render(device.Name)
		rows = append(rows, row)
	}
	body := strings.Join(rows, "\n")
	b.WriteString(Panel("Select GPUs", body, panelWidth, AccentColor()))
	b.WriteString("\n\n")

	if m.status != "" {
		b.WriteString("  ")
		b.WriteString(MutedStyle.Render(m.status))
		b.WriteString("\n\n")
	}
	b.WriteString("  ")
	b.WriteString(HelpBar(
		HelpItem{Key: "↑↓", Desc: "move"},
		HelpItem{Key: "space", Desc: "toggle"},
		HelpItem{Key: "a", Desc: "toggle all"},
		HelpItem{Key: "Enter", Desc: "start"},
		HelpItem{Key: "q", Desc: "cancel"},
	))
	b.WriteString("\n")
	return b.String()
}

func (m gpuSelectModel) countSelected() int {
	count := 0
	for _, selected := range m.selected {
		if selected {
			count++
		}
	}
	return count
}
