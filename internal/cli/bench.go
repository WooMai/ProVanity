package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/woomai/provanity/internal/config"
	"github.com/woomai/provanity/internal/gpu"
	"github.com/woomai/provanity/internal/human"
	"github.com/woomai/provanity/internal/local"
	"golang.org/x/term"
)

func explicitCUDAParams(cmd *cobra.Command, batchMultiple, workSize int) bool {
	return (cmd.Flags().Changed("batch-multiple") && batchMultiple > 0) ||
		(cmd.Flags().Changed("work-size") && workSize > 0)
}

func newBenchCommand() *cobra.Command {
	var devices string
	var duration time.Duration
	var batchMultiple int
	var workSize int
	var progressInterval int
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Benchmark local CUDA EVM address throughput",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			if err := paths.Ensure(); err != nil {
				return err
			}

			deviceIDs, requestedSelection, err := local.ParseDeviceIDs(devices)
			if err != nil {
				return err
			}
			if requestedSelection {
				return fmt.Errorf("bench does not support interactive device selection; pass --devices with explicit ids")
			}
			if duration <= 0 {
				return fmt.Errorf("duration must be positive")
			}
			if batchMultiple < 0 {
				return fmt.Errorf("batch multiple cannot be negative")
			}
			if workSize < 0 {
				return fmt.Errorf("work size cannot be negative")
			}
			if progressInterval <= 0 {
				return fmt.Errorf("progress interval must be positive")
			}

			out := cmd.OutOrStdout()
			live := newBenchmarkLiveRenderer(out, !jsonOutput, duration, devices)
			live.Start()
			result, err := local.Benchmark(cmd.Context(), local.BenchmarkOptions{
				DeviceIDs:          deviceIDs,
				BatchMultiple:      batchMultiple,
				WorkSize:           workSize,
				ManualParams:       explicitCUDAParams(cmd, batchMultiple, workSize),
				ProgressIntervalMS: progressInterval,
				Duration:           duration,
			}, live.Emit)
			if err != nil {
				live.Fail(err)
				if jsonOutput {
					// Surface the error in machine-readable form on stdout so
					// callers parsing JSON output see a structured failure
					// (e.g. CI pipelines that consume bench JSON).
					encoder := json.NewEncoder(out)
					encoder.SetIndent("", "  ")
					_ = encoder.Encode(map[string]string{"error": err.Error()})
				}
				return err
			}
			if jsonOutput {
				encoder := json.NewEncoder(out)
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			}

			live.Finish(result)
			return nil
		},
	}

	cmd.Flags().StringVar(&devices, "devices", "all", "comma-separated CUDA device ids or all")
	cmd.Flags().DurationVar(&duration, "duration", 10*time.Second, "benchmark duration after the first progress sample")
	cmd.Flags().IntVarP(&batchMultiple, "batch-multiple", "B", 0, "advanced raw GPU batch size; omit for benchmark autotune")
	cmd.Flags().IntVar(&workSize, "work-size", 0, "CUDA threads per block; omit for benchmark autotune")
	cmd.Flags().IntVar(&progressInterval, "progress-interval", 1000, "GPU progress interval in milliseconds")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print final benchmark result as JSON")
	return cmd
}

type benchmarkLiveRenderer struct {
	out              io.Writer
	quiet            bool
	live             bool
	color            bool
	duration         time.Duration
	requestedDevices string
	started          time.Time
	lines            int
	spinner          int
	closed           bool

	status         string
	system         local.BenchmarkSystemInfo
	devices        []gpu.Device
	params         local.CUDAParams
	paramSource    string
	tuningState    string
	tuningPhase    string
	tuningIndex    int
	tuningTotal    int
	tuningParams   local.CUDAParams
	tuningHashrate uint64
	tuningElapsed  uint64
	tuningDuration float64
	topTuning      []local.TuningSample
	elapsedSec     uint64
	attempts       uint64
	hashrate       uint64
	peakHashrate   uint64
	samples        int
	finished       bool
	err            error
}

func newBenchmarkLiveRenderer(out io.Writer, enabled bool, duration time.Duration, requestedDevices string) *benchmarkLiveRenderer {
	live := enabled && isTerminalWriter(out)
	return &benchmarkLiveRenderer{
		out:              out,
		quiet:            !enabled,
		live:             live,
		color:            live && os.Getenv("NO_COLOR") == "",
		duration:         duration,
		requestedDevices: requestedDevices,
		status:           "starting",
	}
}

func isTerminalWriter(out io.Writer) bool {
	file, ok := out.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func (r *benchmarkLiveRenderer) Start() {
	if r.quiet {
		return
	}
	r.started = time.Now()
	if r.live {
		fmt.Fprint(r.out, "\x1b[?25l")
		r.render()
	}
}

func (r *benchmarkLiveRenderer) Emit(event local.BenchmarkEvent) {
	if r.quiet || r.closed {
		return
	}
	if event.Tuning != nil {
		r.updateTuning(*event.Tuning)
	}
	if event.GPUEvent != nil {
		r.updateGPU(event.GPUEvent)
	}
	if r.live {
		r.render()
	}
}

func (r *benchmarkLiveRenderer) Finish(result local.BenchmarkResult) {
	if r.quiet || r.closed {
		return
	}
	r.finished = true
	r.status = "complete"
	r.system = result.System
	r.devices = result.Devices
	r.params = result.Params
	r.paramSource = result.ParamSource
	r.elapsedSec = result.ElapsedSec
	r.attempts = result.Attempts
	r.hashrate = result.Hashrate
	r.peakHashrate = result.PeakHashrate
	r.samples = result.Samples
	r.topTuning = result.TopTuning
	if len(r.topTuning) == 0 {
		r.topTuning = result.Tuning
	}
	if r.live {
		r.render()
		r.closeLive()
		return
	}
	printBenchmarkResult(r.out, result)
	r.closed = true
}

func (r *benchmarkLiveRenderer) Fail(err error) {
	if r.quiet || r.closed {
		return
	}
	r.err = err
	r.status = "error"
	if r.live {
		r.render()
		r.closeLive()
		return
	}
	fmt.Fprintf(r.out, "benchmark error: %v\n", err)
	r.closed = true
}

func (r *benchmarkLiveRenderer) updateTuning(event local.TuningEvent) {
	r.tuningState = event.State
	r.tuningPhase = event.Phase
	r.tuningIndex = event.Index
	r.tuningTotal = event.Total
	r.tuningParams = event.Params
	r.tuningElapsed = event.ElapsedSec
	r.tuningDuration = event.DurationSec
	if event.Hashrate > 0 {
		r.tuningHashrate = event.Hashrate
		if event.Hashrate > r.peakHashrate {
			r.peakHashrate = event.Hashrate
		}
	}
	switch event.State {
	case local.TuningStateActive:
		r.status = "autotune"
	case local.TuningStateSampled:
		r.status = "sampled"
		if event.Hashrate > 0 {
			r.topTuning = append(r.topTuning, local.TuningSample{
				Params:   event.Params,
				Hashrate: event.Hashrate,
				Round:    event.Phase,
			})
		}
	case local.TuningStateSelected:
		r.status = "selected"
		r.params = event.Params
		r.paramSource = "autotune"
	case local.TuningStateDefault:
		r.status = "default params"
	case local.TuningStateSkipped:
		r.status = "skipped"
	}
}

func (r *benchmarkLiveRenderer) updateGPU(event gpu.Event) {
	switch e := event.(type) {
	case gpu.ReadyEvent:
		r.devices = e.Devices
		if r.status == "starting" {
			r.status = "warming up"
		}
	case gpu.ProgressEvent:
		r.elapsedSec = e.ElapsedSec
		r.attempts = e.Attempts
		r.hashrate = e.Hashrate
		if e.Hashrate > r.peakHashrate {
			r.peakHashrate = e.Hashrate
		}
		if len(e.Devices) > 0 {
			r.devices = e.Devices
		}
		if r.status == "starting" || r.status == "warming up" || r.status == "selected" || r.status == "default params" {
			r.status = "benchmarking"
		}
		r.samples++
	case gpu.ErrorEvent:
		r.err = fmt.Errorf("%s", e.Message)
		r.status = "error"
	}
}

func (r *benchmarkLiveRenderer) render() {
	panel := r.panel()
	if r.lines > 0 {
		fmt.Fprintf(r.out, "\x1b[%dA\x1b[J", r.lines)
	}
	fmt.Fprint(r.out, panel)
	r.lines = strings.Count(panel, "\n")
}

func (r *benchmarkLiveRenderer) closeLive() {
	fmt.Fprint(r.out, "\x1b[?25h")
	r.closed = true
}

func (r *benchmarkLiveRenderer) panel() string {
	var b strings.Builder
	status := r.status
	if r.err != nil {
		status = "error"
	}
	if r.finished {
		status = "complete"
	}
	spinner := r.spinnerFrame(status)
	fmt.Fprintf(&b, "%s %s %s\n", r.paint("PROVANITY BENCH", ansiBold+ansiMagenta), spinner, r.statusBadge(status))
	fmt.Fprintf(&b, "%s %s    %s %s\n",
		r.paint(r.hashrateLabel(), ansiDim),
		r.paint(human.FormatHashrate(r.displayHashrate()), ansiBold+ansiCyan),
		r.paint("peak", ansiDim),
		r.paint(human.FormatHashrate(r.peakHashrate), ansiBold+ansiGreen),
	)
	if r.isAutotuning() {
		// Show live progress on top (attempts keep ticking because every
		// autotune probe runs the real search kernel — the work isn't wasted)
		// and the sample/stage info on a second line.
		fmt.Fprintf(&b, "%s %s / %s    %s %d\n",
			r.paint("elapsed", ansiDim),
			formatSeconds(r.displayElapsedSec()),
			formatDurationSeconds(r.duration),
			r.paint("attempts", ansiDim),
			r.attempts,
		)
		fmt.Fprintf(&b, "%s %s / %s    %s %s\n",
			r.paint("sample", ansiDim),
			formatSeconds(r.tuningElapsed),
			formatDurationSeconds(time.Duration(r.tuningDuration*float64(time.Second))),
			r.paint("stage", ansiDim),
			r.tuningLabel(),
		)
	} else {
		fmt.Fprintf(&b, "%s %s / %s    %s %d\n",
			r.paint("elapsed", ansiDim),
			formatSeconds(r.displayElapsedSec()),
			formatDurationSeconds(r.duration),
			r.paint("attempts", ansiDim),
			r.attempts,
		)
	}
	fmt.Fprintf(&b, "%s %s %s\n", r.paint("progress", ansiDim), r.progressBar(34), r.progressLabel())
	if r.tuningState != "" && !r.finished {
		fmt.Fprintf(&b, "%s %s\n",
			r.paint("autotune", ansiDim),
			r.tuningDescription(),
		)
		fmt.Fprintf(&b, "%s %s\n", r.paint("params", ansiDim), r.paint(formatCUDAParams(r.tuningParams), ansiYellow))
	} else if r.params.BatchMultiple > 0 || r.params.WorkSize > 0 || r.paramSource != "" {
		source := r.paramSource
		if source == "" {
			source = "manual"
		}
		fmt.Fprintf(&b, "%s %s    %s %s\n",
			r.paint("params", ansiDim),
			r.paint(formatCUDAParams(r.params), ansiYellow),
			r.paint("source", ansiDim),
			source,
		)
	} else {
		fmt.Fprintf(&b, "%s %s\n", r.paint("devices", ansiDim), r.requestedDevices)
	}
	if r.finished && (r.system.OS != "" || r.system.Arch != "" || r.system.DriverVersion != "") {
		system := strings.Trim(strings.Join([]string{r.system.OS, r.system.Arch}, "/"), "/")
		if system == "" {
			system = "unknown"
		}
		if r.system.DriverVersion != "" {
			fmt.Fprintf(&b, "%s %s    %s %s\n", r.paint("system", ansiDim), system, r.paint("driver", ansiDim), r.system.DriverVersion)
		} else {
			fmt.Fprintf(&b, "%s %s\n", r.paint("system", ansiDim), system)
		}
	}
	if len(r.devices) > 0 {
		fmt.Fprintf(&b, "%s\n", r.paint("GPUs", ansiBold))
		for _, device := range r.devices {
			fmt.Fprintf(&b, "  %s\n", formatBenchmarkDevice(device, !r.finished))
		}
	}
	top := topBenchmarkTuningSamples(r.topTuning, 3)
	if len(top) > 0 {
		fmt.Fprintf(&b, "%s\n", r.paint("Top tuning params", ansiBold))
		for i, sample := range top {
			fmt.Fprintf(&b, "  %2d. %-44s %s\n",
				i+1,
				formatCUDAParams(sample.Params),
				r.paint(human.FormatHashrate(sample.Hashrate), ansiGreen),
			)
		}
	}
	if r.err != nil {
		fmt.Fprintf(&b, "%s %v\n", r.paint("error", ansiRed+ansiBold), r.err)
	}
	return b.String()
}

func (r *benchmarkLiveRenderer) displayHashrate() uint64 {
	if r.hashrate > 0 {
		return r.hashrate
	}
	return r.tuningHashrate
}

func (r *benchmarkLiveRenderer) hashrateLabel() string {
	if r.finished {
		return "avg hashrate"
	}
	if r.isAutotuning() {
		return "sample rate"
	}
	return "hashrate"
}

func (r *benchmarkLiveRenderer) displayElapsedSec() uint64 {
	if r.elapsedSec > 0 || r.started.IsZero() {
		return r.elapsedSec
	}
	return uint64(time.Since(r.started).Seconds())
}

func (r *benchmarkLiveRenderer) progressRatio() float64 {
	if r.finished {
		return 1
	}
	if r.isAutotuning() {
		if r.tuningTotal <= 0 {
			return 0
		}
		sampleRatio := 0.0
		if r.tuningDuration > 0 {
			sampleRatio = float64(r.tuningElapsed) / r.tuningDuration
			if sampleRatio > 1 {
				sampleRatio = 1
			}
		}
		switch r.tuningState {
		case local.TuningStateSampled, local.TuningStateSkipped:
			sampleRatio = 1
		}
		ratio := (float64(maxInt(0, r.tuningIndex-1)) + sampleRatio) / float64(r.tuningTotal)
		return clampRatio(ratio)
	}
	if r.duration <= 0 {
		return 0
	}
	return clampRatio(float64(r.displayElapsedSec()) / r.duration.Seconds())
}

func (r *benchmarkLiveRenderer) isAutotuning() bool {
	if r.finished {
		return false
	}
	switch r.tuningState {
	case local.TuningStateActive, local.TuningStateSampled, local.TuningStateSkipped:
		return true
	default:
		return false
	}
}

func (r *benchmarkLiveRenderer) progressBar(width int) string {
	if width <= 0 {
		return ""
	}
	ratio := r.progressRatio()
	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("#", filled) + strings.Repeat("-", width-filled)
	return "[" + r.paint(bar[:filled], ansiMagenta+ansiBold) + r.paint(bar[filled:], ansiDim) + "]"
}

func (r *benchmarkLiveRenderer) progressLabel() string {
	ratio := r.progressRatio()
	label := fmt.Sprintf("%3.0f%%", ratio*100)
	if r.isAutotuning() {
		return r.paint(label+" tune", ansiYellow)
	}
	if r.finished {
		return r.paint(label+" done", ansiGreen)
	}
	return r.paint(label+" bench", ansiCyan)
}

func (r *benchmarkLiveRenderer) spinnerFrame(status string) string {
	if r.finished {
		return r.paint("ok", ansiGreen+ansiBold)
	}
	if r.err != nil || status == "error" {
		return r.paint("!!", ansiRed+ansiBold)
	}
	frames := []string{"-", "\\", "|", "/"}
	frame := frames[r.spinner%len(frames)]
	r.spinner++
	return r.paint(frame, ansiCyan+ansiBold)
}

func (r *benchmarkLiveRenderer) statusBadge(status string) string {
	style := ansiCyan + ansiBold
	switch status {
	case "complete", "selected":
		style = ansiGreen + ansiBold
	case "autotune", "sampled", "skipped", "default params":
		style = ansiYellow + ansiBold
	case "error":
		style = ansiRed + ansiBold
	}
	return r.paint(strings.ToUpper(status), style)
}

func (r *benchmarkLiveRenderer) tuningLabel() string {
	label := r.tuningState
	if r.tuningPhase != "" {
		label = r.tuningPhase
	}
	if r.tuningTotal > 0 {
		label += fmt.Sprintf(" %d/%d", r.tuningIndex, r.tuningTotal)
	}
	return r.paint(label, ansiYellow)
}

func (r *benchmarkLiveRenderer) tuningDescription() string {
	switch r.tuningPhase {
	case "probe":
		return "probing CUDA params — live rate may not reflect peak"
	case "confirm":
		totalParams := r.tuningTotal / 2
		if totalParams > 0 {
			return fmt.Sprintf("confirming top %d params twice — live rate may not reflect peak", totalParams)
		}
		return "confirming top CUDA parameter candidates"
	default:
		return "selecting CUDA parameters"
	}
}

func (r *benchmarkLiveRenderer) paint(value, style string) string {
	if !r.color || value == "" {
		return value
	}
	return style + value + ansiReset
}

func printBenchmarkResult(out io.Writer, result local.BenchmarkResult) {
	fmt.Fprintln(out, "Benchmark complete")
	if result.System.OS != "" || result.System.Arch != "" {
		fmt.Fprintf(out, "system: %s/%s\n", result.System.OS, result.System.Arch)
	}
	if result.System.DriverVersion != "" {
		fmt.Fprintf(out, "driver: %s\n", result.System.DriverVersion)
	}
	if result.ParamSource != "" && (result.Params.BatchMultiple > 0 || result.Params.WorkSize > 0 || result.ParamSource != "manual") {
		fmt.Fprintf(out, "params: batch_multiple=%d work_size=%d source=%s\n",
			result.Params.BatchMultiple, result.Params.WorkSize, result.ParamSource)
	}
	fmt.Fprintf(out, "duration: %.1fs\n", result.DurationSec)
	fmt.Fprintf(out, "elapsed: %s\n", formatSeconds(result.ElapsedSec))
	fmt.Fprintf(out, "attempts: %d\n", result.Attempts)
	fmt.Fprintf(out, "average hashrate: %s\n", human.FormatHashrate(result.Hashrate))
	fmt.Fprintf(out, "peak hashrate: %s\n", human.FormatHashrate(result.PeakHashrate))
	fmt.Fprintf(out, "samples: %d\n", result.Samples)
	if len(result.Devices) > 0 {
		fmt.Fprintln(out, "devices:")
		for _, device := range result.Devices {
			fmt.Fprintf(out, "  %s\n", formatBenchmarkDevice(device, false))
		}
	}
	topTuning := result.TopTuning
	if len(topTuning) == 0 {
		topTuning = result.Tuning
	}
	printBenchmarkTopTuning(out, topTuning)
}

func printBenchmarkTopTuning(out io.Writer, samples []local.TuningSample) {
	top := topBenchmarkTuningSamples(samples, 3)
	if len(top) == 0 {
		return
	}
	fmt.Fprintln(out, "top tuning params:")
	for i, sample := range top {
		fmt.Fprintf(out, "  #%d %s hashrate=%s\n",
			i+1,
			formatCUDAParams(sample.Params),
			human.FormatHashrate(sample.Hashrate),
		)
	}
}

func topBenchmarkTuningSamples(samples []local.TuningSample, limit int) []local.TuningSample {
	if limit <= 0 {
		return nil
	}
	type aggregate struct {
		params local.CUDAParams
		rates  []uint64
	}
	byParams := make(map[local.CUDAParams]*aggregate)
	for _, sample := range samples {
		if sample.Hashrate == 0 {
			continue
		}
		agg := byParams[sample.Params]
		if agg == nil {
			agg = &aggregate{params: sample.Params}
			byParams[sample.Params] = agg
		}
		agg.rates = append(agg.rates, sample.Hashrate)
	}
	top := make([]local.TuningSample, 0, len(byParams))
	for _, agg := range byParams {
		top = append(top, local.TuningSample{
			Params:   agg.params,
			Hashrate: medianHashrate(agg.rates),
		})
	}
	sort.SliceStable(top, func(i, j int) bool {
		return top[i].Hashrate > top[j].Hashrate
	})
	if len(top) > limit {
		top = top[:limit]
	}
	return top
}

func medianHashrate(values []uint64) uint64 {
	if len(values) == 0 {
		return 0
	}
	values = append([]uint64(nil), values...)
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values[(len(values)-1)/2]
}

func formatBenchmarkDevice(device gpu.Device, includeHashrate bool) string {
	label := fmt.Sprintf("GPU%d", device.ID)
	if device.Name != "" {
		label += " " + device.Name
	}
	if device.GlobalMem > 0 {
		label += fmt.Sprintf(" (%d MiB)", device.GlobalMem/1024/1024)
	}
	if includeHashrate && device.Hashrate > 0 {
		label += ": " + human.FormatHashrate(device.Hashrate)
	}
	return label
}

func formatCUDAParams(params local.CUDAParams) string {
	if params.BatchMultiple <= 0 && params.WorkSize <= 0 {
		return "auto"
	}
	return fmt.Sprintf("batch_multiple=%d work_size=%d", params.BatchMultiple, params.WorkSize)
}

func formatDurationSeconds(duration time.Duration) string {
	if duration <= 0 {
		return "00:00"
	}
	return formatSeconds(uint64(duration.Seconds()))
}

func clampRatio(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

const (
	ansiReset   = "\x1b[0m"
	ansiBold    = "\x1b[1m"
	ansiDim     = "\x1b[2m"
	ansiRed     = "\x1b[31m"
	ansiGreen   = "\x1b[32m"
	ansiYellow  = "\x1b[33m"
	ansiMagenta = "\x1b[35m"
	ansiCyan    = "\x1b[36m"
)
