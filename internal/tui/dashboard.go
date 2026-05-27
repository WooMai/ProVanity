package tui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/woomai/provanity/internal/estimate"
	"github.com/woomai/provanity/internal/gpu"
	"github.com/woomai/provanity/internal/human"
	"github.com/woomai/provanity/internal/local"
)

type RunFunc func(context.Context, local.EmitFunc) (local.Result, error)

type DashboardOptions struct {
	Pattern string
	Devices string
}

type dashboardDone struct {
	result local.Result
	err    error
}

type eventMsg local.RunEvent
type eventsClosedMsg struct{}
type doneMsg dashboardDone

type dashboardModel struct {
	opts          DashboardOptions
	ctx           context.Context
	run           RunFunc
	events        <-chan local.RunEvent
	eventSink     chan<- local.RunEvent
	done          <-chan dashboardDone
	doneSink      chan<- dashboardDone
	cancel        context.CancelFunc
	width         int
	height        int
	status        string
	showHelp      bool
	showList      bool
	stopping      bool
	completed     bool
	searchStarted bool
	err           error
	result        local.Result

	devices      []gpu.Device
	progressByID map[int]gpu.Device
	progress     gpu.ProgressEvent
	peakHashrate uint64
	tuning       local.TuningEvent
	candidates   []local.Candidate
	startedAt    time.Time

	spinner spinner.Model
	bar     progress.Model
}

func RunDashboard(ctx context.Context, opts DashboardOptions, run RunFunc) (local.Result, error) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	events := make(chan local.RunEvent, 64)
	done := make(chan dashboardDone, 1)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(PinkColor()).Bold(true)

	bar := progress.New(
		progress.WithGradient(string(PinkColor()), string(AccentColor())),
		progress.WithoutPercentage(),
		progress.WithWidth(28),
	)

	model := dashboardModel{
		opts:         opts,
		ctx:          runCtx,
		run:          run,
		events:       events,
		eventSink:    events,
		done:         done,
		doneSink:     done,
		cancel:       cancel,
		status:       "warming up the CUDA backend",
		progressByID: make(map[int]gpu.Device),
		startedAt:    time.Now(),
		spinner:      sp,
		bar:          bar,
	}

	program := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
		return local.Result{}, err
	}

	final := finalModel.(dashboardModel)
	if final.result.Address == "" {
		cancel()
		select {
		case msg := <-done:
			if msg.result.Address != "" {
				return msg.result, nil
			}
			if msg.err != nil {
				return local.Result{}, msg.err
			}
		case <-time.After(2 * time.Second):
		}
	}
	if final.err != nil && final.result.Address == "" {
		return local.Result{}, final.err
	}
	return final.result, nil
}

func startRun(ctx context.Context, run RunFunc, events chan<- local.RunEvent, done chan<- dashboardDone) tea.Cmd {
	return func() tea.Msg {
		go func() {
			emit := func(event local.RunEvent) {
				select {
				case events <- event:
				case <-ctx.Done():
				}
			}
			result, err := run(ctx, emit)
			close(events)
			done <- dashboardDone{result: result, err: err}
		}()
		return nil
	}
}

func waitEvent(events <-chan local.RunEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return eventsClosedMsg{}
		}
		return eventMsg(event)
	}
}

func waitDone(done <-chan dashboardDone) tea.Cmd {
	return func() tea.Msg {
		return doneMsg(<-done)
	}
}

func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(
		startRun(m.ctx, m.run, m.eventSink, m.doneSink),
		waitEvent(m.events),
		waitDone(m.done),
		m.spinner.Tick,
	)
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		barWidth := m.width / 3
		if barWidth < 18 {
			barWidth = 18
		}
		if barWidth > 36 {
			barWidth = 36
		}
		m.bar.Width = barWidth
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if !m.completed && !m.stopping {
				m.stopping = true
				m.status = "stopping CUDA — preserving the best candidate so far"
				m.cancel()
				return m, nil
			}
			return m, tea.Quit
		case "p":
			if !m.completed && !m.stopping {
				m.stopping = true
				m.status = "stop requested — preserving the best candidate so far"
				m.cancel()
			}
			return m, nil
		case "s":
			m.showList = !m.showList
			return m, nil
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "c":
			text := m.copyText()
			if text == "" {
				m.status = "nothing to copy yet"
				return m, nil
			}
			m.status = "copied latest address via OSC52"
			return m, tea.Printf("%s", osc52.New(text))
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		updated, cmd := m.bar.Update(msg)
		m.bar = updated.(progress.Model)
		return m, cmd
	case eventMsg:
		m.applyEvent(local.RunEvent(msg))
		return m, waitEvent(m.events)
	case eventsClosedMsg:
		return m, nil
	case doneMsg:
		m.completed = true
		m.result = msg.result
		m.err = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m dashboardModel) copyText() string {
	if m.result.Address != "" {
		return m.result.Address
	}
	if len(m.candidates) > 0 {
		return m.candidates[0].Address
	}
	return ""
}

func (m *dashboardModel) applyEvent(event local.RunEvent) {
	if event.Message != "" {
		m.status = event.Message
	}
	if event.Tuning != nil {
		m.tuning = *event.Tuning
		m.status = m.tuningStatus()
	}
	if event.Candidate != nil {
		m.searchStarted = true
		m.candidates = append([]local.Candidate{*event.Candidate}, m.candidates...)
		if len(m.candidates) > 8 {
			m.candidates = m.candidates[:8]
		}
		if event.Candidate.Matched {
			m.status = "target reached — verifying the key locally"
		} else {
			m.status = fmt.Sprintf("score improved to %s — verifying", formatDashboardScore(event.Candidate.Score, event.Candidate.TargetScore))
		}
	}
	if event.Result != nil {
		m.searchStarted = true
		m.result = *event.Result
		if event.Result.Partial {
			m.status = "best partial candidate verified locally"
		} else {
			m.status = "target reached and verified locally"
		}
	}
	if event.GPUEvent == nil {
		return
	}

	switch e := event.GPUEvent.(type) {
	case gpu.ReadyEvent:
		m.devices = e.Devices
		m.status = "GPU ready — building tables, first batch"
	case gpu.PhaseEvent:
		m.status = e.Message
	case gpu.ProgressEvent:
		m.searchStarted = true
		m.progress = e
		if e.Hashrate > m.peakHashrate {
			m.peakHashrate = e.Hashrate
		}
		for _, device := range e.Devices {
			m.progressByID[device.ID] = device
		}
		if m.status == "" ||
			strings.HasPrefix(m.status, "GPU ready") ||
			strings.HasPrefix(m.status, "warming") ||
			strings.HasPrefix(m.status, "CUDA params selected") ||
			isPhaseStatus(m.status) {
			m.status = m.searchStatus()
		}
	case gpu.FoundEvent:
		m.searchStarted = true
		m.status = "GPU found candidate " + shortAddress(e.Address)
	case gpu.ErrorEvent:
		m.status = e.Message
	}
}

// isPhaseStatus reports whether the current status line was last set by a
// PhaseEvent. The first ProgressEvent uses this to know it's safe to clear
// the setup message and switch to the live search status. Match on the
// message prefixes emitted by emit_phase in backend.cu.
func isPhaseStatus(status string) bool {
	return strings.HasPrefix(status, "preparing CUDA") ||
		strings.HasPrefix(status, "allocating ") ||
		strings.HasPrefix(status, "building secp256k1") ||
		strings.HasPrefix(status, "initializing ")
}

func (m dashboardModel) View() string {
	width := m.width
	if width <= 0 {
		width = 96
	}
	if width < 70 {
		width = 70
	}
	if width > 120 {
		width = 120
	}
	panelWidth := width - 4
	if panelWidth < 60 {
		panelWidth = 60
	}

	var b strings.Builder
	b.WriteString(m.headerView(panelWidth))
	b.WriteString("\n\n")

	b.WriteString(m.bestView(panelWidth))
	b.WriteString("\n\n")
	b.WriteString(m.statsView(panelWidth))
	b.WriteString("\n\n")
	b.WriteString(m.devicesView(panelWidth))
	b.WriteString("\n\n")
	if m.showList {
		b.WriteString(m.candidatesView(panelWidth))
		b.WriteString("\n\n")
	}
	if m.showHelp {
		b.WriteString(m.helpView(panelWidth))
		b.WriteString("\n\n")
	}
	b.WriteString("  ")
	b.WriteString(m.statusLine())
	b.WriteString("\n\n")
	b.WriteString("  ")
	b.WriteString(m.footerView())
	b.WriteString("\n")
	return b.String()
}

func (m dashboardModel) headerView(width int) string {
	var statusBadge string
	switch {
	case m.completed && m.err != nil && m.result.Address == "":
		statusBadge = Badge(" ERROR ", "#0B0B14", string(RedColor()))
	case m.completed && m.result.Partial:
		statusBadge = Badge(" STOPPED ", "#0B0B14", string(YellowColor()))
	case m.completed && m.result.Address != "":
		statusBadge = Badge(" MATCHED ", "#0B0B14", string(GreenColor()))
	case m.stopping:
		statusBadge = Badge(" STOPPING ", "#0B0B14", string(YellowColor()))
	case m.warmingUp():
		statusBadge = m.spinner.View() + " " + YellowStyle.Render("WARMING UP")
	case m.isAutotuning():
		statusBadge = m.spinner.View() + " " + YellowStyle.Render("AUTOTUNING")
	case m.progress.HashrateUncertain:
		statusBadge = m.spinner.View() + " " + YellowStyle.Render("SEARCHING*")
	default:
		statusBadge = m.spinner.View() + " " + AccentStyle.Render("SEARCHING")
	}

	innerWidth := width - 4
	if innerWidth < 20 {
		innerWidth = 20
	}
	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(innerWidth/2).Render(Logo()),
		lipgloss.NewStyle().Width(innerWidth-innerWidth/2).Align(lipgloss.Right).Render(statusBadge),
	)
	meta := fmt.Sprintf("pattern %s  ·  devices %s", m.opts.Pattern, m.opts.Devices)
	metaRow := MutedStyle.Render(truncate(meta, innerWidth))
	return Panel("", topRow+"\n"+metaRow, width, AccentColor())
}

func (m dashboardModel) statsView(width int) string {
	peakHashrate := m.peakHashrate
	if m.progress.Hashrate > peakHashrate {
		peakHashrate = m.progress.Hashrate
	}
	rows := []string{
		statsRow("current", formatHashrate(m.progress.Hashrate)+tuningMarker(m.progress.HashrateUncertain)),
		statsRow("peak", formatHashrate(peakHashrate)+tuningMarker(m.progress.HashrateUncertain)),
		statsRow("elapsed", formatDuration(m.progress.ElapsedSec)),
		statsRow("attempts", formatCount(m.progress.Attempts)),
	}
	if tuning := m.tuningStatsRows(); len(tuning) > 0 {
		rows = append(rows, tuning...)
	}
	return Panel("STATS", strings.Join(rows, "\n"), width, AccentColor())
}

func (m dashboardModel) isAutotuning() bool {
	return m.tuning.State == local.TuningStateActive
}

// warmingUp is the window between launch and the first real search signal:
// the GPU is building precompute tables and running the first batch, so no
// hashrate/attempts exist yet. Autotuning has its own status, so it is excluded.
func (m dashboardModel) warmingUp() bool {
	return !m.completed && !m.stopping && !m.searchStarted && !m.isAutotuning()
}

func (m dashboardModel) searchStatus() string {
	if m.isAutotuning() {
		return m.tuningStatus()
	}
	if m.progress.HashrateUncertain {
		return "searching with provisional CUDA params"
	}
	if m.tuning.State == local.TuningStateSelected {
		if params := formatCUDAParams(m.tuning.Params); params != "" {
			return "searching with tuned CUDA params: " + params
		}
		return "searching with tuned CUDA params"
	}
	return "searching"
}

func (m dashboardModel) tuningStatus() string {
	switch m.tuning.State {
	case local.TuningStateActive:
		stage := formatTuningStage(m.tuning)
		params := formatCUDAParams(m.tuning.Params)
		if stage != "" && params != "" {
			return fmt.Sprintf("autotuning CUDA params: %s %s", stage, params)
		}
		if stage != "" {
			return "autotuning CUDA params: " + stage
		}
		return "autotuning CUDA params"
	case local.TuningStateSelected:
		params := formatCUDAParams(m.tuning.Params)
		if m.tuning.Hashrate > 0 && params != "" {
			return fmt.Sprintf("CUDA params selected: %s at %s", params, formatHashrate(m.tuning.Hashrate))
		}
		if params != "" {
			return "CUDA params selected: " + params
		}
		return "CUDA params selected"
	case local.TuningStateDefault:
		if params := formatCUDAParams(m.tuning.Params); params != "" {
			return "searching with baseline CUDA params: " + params
		}
		return "searching with baseline CUDA params"
	default:
		return m.status
	}
}

func (m dashboardModel) tuningStatsRows() []string {
	paramsRow := func(value string) string {
		row := statsRow("params", value)
		if m.progress.HashrateUncertain {
			row += " " + DimStyle.Render("("+m.hashrateUncertainNote()+")")
		}
		return row
	}
	switch m.tuning.State {
	case local.TuningStateActive:
		rows := []string{statsRow("tuning", formatTuningStage(m.tuning))}
		if params := formatCUDAParams(m.tuning.Params); params != "" {
			rows = append(rows, paramsRow(params))
		}
		return rows
	case local.TuningStateSelected:
		if params := formatCUDAParams(m.tuning.Params); params != "" {
			return []string{paramsRow(params)}
		}
	case local.TuningStateDefault:
		if params := formatCUDAParams(m.tuning.Params); params != "" {
			return []string{paramsRow(params)}
		}
	}
	if m.progress.HashrateUncertain {
		return []string{paramsRow("provisional")}
	}
	return nil
}

func (m dashboardModel) hashrateUncertainNote() string {
	switch m.tuning.State {
	case local.TuningStateActive:
		return "autotuning CUDA params; speed may change"
	case local.TuningStateDefault:
		return "baseline CUDA params; speed may be below tuned"
	default:
		return "CUDA params not finalized; speed may change"
	}
}

func formatTuningStage(tuning local.TuningEvent) string {
	phase := tuning.Phase
	if phase == "" {
		phase = "probe"
	}
	if tuning.Index > 0 && tuning.Total > 0 {
		return fmt.Sprintf("%s %d/%d", phase, tuning.Index, tuning.Total)
	}
	return phase
}

func formatCUDAParams(params local.CUDAParams) string {
	if params.BatchMultiple <= 0 || params.WorkSize <= 0 {
		return ""
	}
	return fmt.Sprintf("B=%d W=%d", params.BatchMultiple, params.WorkSize)
}

func statsRow(label, value string) string {
	return MutedStyle.Render(padRightTUI(label, 10)) + " " + StrongStyle.Bold(true).Render(value)
}

func (m dashboardModel) bestView(width int) string {
	if m.result.Address == "" && len(m.candidates) == 0 {
		body := DimStyle.Render("no candidate yet — keep waiting")
		return Panel("BEST", body, width, PinkColor())
	}

	var (
		address     string
		score       int
		target      int
		isPartial   bool
		isMatched   bool
		bestElapsed uint64
	)
	if m.result.Address != "" {
		address = m.result.Address
		score = m.result.Score
		target = m.result.TargetScore
		isPartial = m.result.Partial
		isMatched = !m.result.Partial && address != ""
		bestElapsed = m.result.Stats.ElapsedSec
	} else if len(m.candidates) > 0 {
		address = m.candidates[0].Address
		score = m.candidates[0].Score
		target = m.candidates[0].TargetScore
		isMatched = m.candidates[0].Matched
		bestElapsed = m.candidates[0].ElapsedSec
	}

	var label string
	switch {
	case isMatched:
		label = GreenStyle.Render("● matched")
	case isPartial:
		label = YellowStyle.Render("● partial")
	default:
		label = PinkStyle.Render("● candidate")
	}

	rows := []string{
		label + "  " + MutedStyle.Render("score") + " " + StrongStyle.Bold(true).Render(formatDashboardScore(score, target)),
		MutedStyle.Render(padRightTUI("address", 10)) + " " + StrongStyle.Render(address),
	}
	if bestElapsed > 0 {
		rows = append(rows, MutedStyle.Render(padRightTUI("elapsed", 10))+" "+StrongStyle.Render(formatDuration(bestElapsed)))
	}
	if avgRate := m.bestAverageHashrate(); avgRate > 0 {
		rows = append(rows, MutedStyle.Render(padRightTUI("avg rate", 10))+" "+StrongStyle.Render(formatHashrateFloat(avgRate)))
	}
	if luck := m.bestLuck(); luck != "" {
		rows = append(rows, MutedStyle.Render(padRightTUI("luck", 10))+" "+StrongStyle.Render(luck))
	}
	if !isMatched {
		scoreBase := 16
		if m.result.Address != "" {
			scoreBase = m.result.ScoreBase
		} else if len(m.candidates) > 0 {
			scoreBase = m.candidates[0].ScoreBase
		}
		if eta := m.nextScoreETA(bestElapsed, score, target, scoreBase); eta != "" {
			rows = append(rows, eta)
		}
	}
	return Panel("BEST", strings.Join(rows, "\n"), width, PinkColor())
}

func (m dashboardModel) bestAverageHashrate() float64 {
	if m.result.Address != "" {
		return averageHashrate(m.result.Stats.Attempts, m.result.Stats.ElapsedSec)
	}
	if len(m.candidates) > 0 {
		return averageHashrate(m.candidates[0].Attempts, m.candidates[0].ElapsedSec)
	}
	return 0
}

func (m dashboardModel) bestLuck() string {
	if m.result.Address != "" {
		return formatLuck(m.result.Score, m.result.ScoreBase, m.result.Stats.Attempts, m.result.Stats.ElapsedSec)
	}
	if len(m.candidates) > 0 {
		candidate := m.candidates[0]
		return formatLuck(candidate.Score, candidate.ScoreBase, candidate.Attempts, candidate.ElapsedSec)
	}
	return ""
}

func (m dashboardModel) nextScoreETA(bestElapsedSec uint64, score, target, scoreBase int) string {
	if score <= 0 || m.progress.Hashrate == 0 {
		return ""
	}
	if scoreBase <= 1 {
		scoreBase = 16
	}
	elapsedSinceBest := time.Duration(0)
	if m.progress.ElapsedSec > bestElapsedSec {
		elapsedSinceBest = time.Duration(m.progress.ElapsedSec-bestElapsedSec) * time.Second
	}
	next, err := estimate.ForNextScoreBase(score, scoreBase, float64(m.progress.Hashrate), elapsedSinceBest)
	if err != nil {
		return ""
	}
	var status string
	if next.Overdue {
		status = YellowStyle.Render("overdue")
	} else {
		status = StrongStyle.Render(estimate.FormatDuration(next.Remaining))
	}
	ratio := 0.0
	if !next.Overdue {
		full := next.Total.Seconds()
		if full > 0 {
			ratio = 1 - next.Remaining.Seconds()/full
			if ratio < 0 {
				ratio = 0
			}
			if ratio > 1 {
				ratio = 1
			}
		}
	} else {
		ratio = 1
	}
	if score >= target && target > 0 {
		return ""
	}
	bar := m.bar.ViewAs(ratio)
	scoreLabel := fmt.Sprintf("+1 score · %s", next.Quantile)
	return MutedStyle.Render(padRightTUI("eta", 10)) + " " + bar + "\n" +
		MutedStyle.Render(padRightTUI("", 10)) + " " + DimStyle.Render(scoreLabel) + "  " + status
}

func (m dashboardModel) devicesView(width int) string {
	if len(m.devices) == 0 {
		body := m.spinner.View() + " " + DimStyle.Render("probing CUDA devices…")
		return Panel("DEVICES", body, width, AccentColor())
	}
	rows := make([]string, 0, len(m.devices))
	for _, device := range m.devices {
		rate := device.Hashrate
		if progress, ok := m.progressByID[device.ID]; ok {
			rate = progress.Hashrate
		}
		dot := GreenStyle.Render("●")
		if rate == 0 {
			dot = DimStyle.Render("●")
		}
		row := dot + " " +
			AccentStyle.Render(fmt.Sprintf("GPU%d", device.ID)) + "  " +
			StrongStyle.Render(padRightTUI(truncate(device.Name, 40), 40)) + "  " +
			TealStyle.Render(formatHashrate(rate)+tuningMarker(m.progress.HashrateUncertain))
		rows = append(rows, row)
	}
	return Panel("DEVICES", strings.Join(rows, "\n"), width, AccentColor())
}

func (m dashboardModel) candidatesView(width int) string {
	if len(m.candidates) == 0 {
		body := DimStyle.Render("no candidates yet")
		return Panel("CANDIDATES", body, width, PinkColor())
	}
	rows := make([]string, 0, len(m.candidates))
	for _, candidate := range m.candidates {
		marker := PinkStyle.Render("●")
		state := "improved"
		if candidate.Matched {
			marker = GreenStyle.Render("●")
			state = "matched "
		}
		row := marker + " " +
			MutedStyle.Render(padRightTUI(state, 9)) + " " +
			MutedStyle.Render("score") + " " +
			StrongStyle.Bold(true).Render(padRightTUI(formatDashboardScore(candidate.Score, candidate.TargetScore), 6)) + " " +
			StrongStyle.Render(shortAddress(candidate.Address))
		if meta := candidateMeta(candidate); meta != "" {
			row += "  " + DimStyle.Render(meta)
		}
		rows = append(rows, row)
	}
	return Panel("CANDIDATES", strings.Join(rows, "\n"), width, PinkColor())
}

func candidateMeta(candidate local.Candidate) string {
	parts := []string{}
	if candidate.ElapsedSec > 0 {
		parts = append(parts, "time "+formatDuration(candidate.ElapsedSec))
	}
	if luck := formatLuck(candidate.Score, candidate.ScoreBase, candidate.Attempts, candidate.ElapsedSec); luck != "" {
		parts = append(parts, "luck "+luck)
	}
	return strings.Join(parts, " · ")
}

func (m dashboardModel) statusLine() string {
	if m.status == "" {
		return ""
	}
	prefix := m.spinner.View()
	style := StrongStyle
	text := m.status
	switch {
	case m.completed && m.err != nil && m.result.Address == "":
		prefix = RedStyle.Render("✗")
		style = RedStyle
	case m.completed && m.result.Partial:
		prefix = YellowStyle.Render("◐")
		style = YellowStyle
	case m.completed && m.result.Address != "":
		prefix = GreenStyle.Render("✓")
		style = GreenStyle
	case m.stopping:
		prefix = YellowStyle.Render("◐")
		style = YellowStyle
	case m.warmingUp():
		style = YellowStyle
		text = fmt.Sprintf("%s · %ds (one-time GPU setup)", m.status, int(time.Since(m.startedAt).Seconds()))
	case m.isAutotuning() || m.progress.HashrateUncertain:
		style = YellowStyle
	}
	return prefix + "  " + style.Render(text)
}

func (m dashboardModel) helpView(width int) string {
	lines := []string{
		StrongStyle.Render("q") + "  " + DimStyle.Render("exit. While searching, the first press requests a graceful stop."),
		StrongStyle.Render("p") + "  " + DimStyle.Render("stop the search and keep the best candidate so far."),
		StrongStyle.Render("s") + "  " + DimStyle.Render("toggle the candidate list."),
		StrongStyle.Render("c") + "  " + DimStyle.Render("copy the latest address via OSC52."),
	}
	return Panel("HELP", strings.Join(lines, "\n"), width, AccentColor())
}

func (m dashboardModel) footerView() string {
	items := []HelpItem{
		{Key: "s", Desc: "candidates"},
		{Key: "p", Desc: "stop"},
		{Key: "c", Desc: "copy"},
		{Key: "?", Desc: "help"},
		{Key: "q", Desc: "quit"},
	}
	return HelpBar(items...)
}

func formatDashboardScore(score, target int) string {
	if target > 0 {
		return fmt.Sprintf("%d/%d", score, target)
	}
	return fmt.Sprint(score)
}

func formatHashrate(rate uint64) string {
	if rate == 0 {
		return "-"
	}
	return human.FormatHashrate(rate)
}

func formatHashrateFloat(rate float64) string {
	if rate <= 0 {
		return "-"
	}
	return human.FormatHashrateFloat(rate)
}

func averageHashrate(attempts, elapsedSec uint64) float64 {
	if attempts == 0 || elapsedSec == 0 {
		return 0
	}
	return float64(attempts) / float64(elapsedSec)
}

func formatLuck(score, scoreBase int, attempts, elapsedSec uint64) string {
	avgRate := averageHashrate(attempts, elapsedSec)
	if score <= 0 || scoreBase <= 1 || avgRate <= 0 || elapsedSec == 0 {
		return ""
	}
	expectedSec := math.Pow(float64(scoreBase), float64(score)) / avgRate
	if expectedSec <= 0 || math.IsInf(expectedSec, 0) || math.IsNaN(expectedSec) {
		return ""
	}
	return formatLuckRatio(expectedSec / float64(elapsedSec))
}

func formatLuckRatio(ratio float64) string {
	if ratio <= 0 || math.IsInf(ratio, 0) || math.IsNaN(ratio) {
		return ""
	}
	if ratio >= 100 {
		return fmt.Sprintf("%.0fx", ratio)
	}
	if ratio >= 10 {
		return fmt.Sprintf("%.1fx", ratio)
	}
	if ratio >= 1 {
		return fmt.Sprintf("%.2fx", ratio)
	}
	return fmt.Sprintf("%.2fx", ratio)
}

func tuningMarker(uncertain bool) string {
	if uncertain {
		return "*"
	}
	return ""
}

func formatDuration(sec uint64) string {
	d := time.Duration(sec) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func formatCount(value uint64) string {
	if value == 0 {
		return "-"
	}
	if value < 1000 {
		return fmt.Sprint(value)
	}
	units := []string{"", "K", "M", "G", "T", "P"}
	f := float64(value)
	unit := 0
	for f >= 1000 && unit < len(units)-1 {
		f /= 1000
		unit++
	}
	return fmt.Sprintf("%.2f%s", f, units[unit])
}

func shortAddress(value string) string {
	prefix := ""
	if strings.HasPrefix(value, "0x") {
		prefix = "0x"
		value = strings.TrimPrefix(value, "0x")
	}
	if len(value) <= 18 {
		return prefix + value
	}
	return prefix + value[:10] + "..." + value[len(value)-6:]
}

func truncate(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 3 {
		return value[:width]
	}
	return value[:width-3] + "..."
}

func padRightTUI(value string, width int) string {
	runes := []rune(value)
	if len(runes) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(runes))
}
