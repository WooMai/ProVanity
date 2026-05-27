package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/woomai/provanity/internal/cliexit"
	"github.com/woomai/provanity/internal/human"
	"github.com/woomai/provanity/internal/local"
	"github.com/woomai/provanity/internal/tui"
	"github.com/woomai/provanity/internal/vanity"
	"golang.org/x/term"
)

func newGenerateCommand() *cobra.Command {
	var pattern string
	var devices string
	var selectGPU bool

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Run interactive local EVM vanity wallet generation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(cmd, pattern, devices, selectGPU, local.WalletEVM, vanity.ParsePattern)
		},
	}

	cmd.Flags().StringVar(&pattern, "pattern", "", "vanity pattern; for example pattern:dead or leading:0:4")
	cmd.Flags().StringVar(&devices, "devices", "all", "comma-separated CUDA device ids or all")
	cmd.Flags().BoolVar(&selectGPU, "select-gpu", false, "probe and choose GPU devices before starting")
	return cmd
}

func newGenerateTronCommand() *cobra.Command {
	var pattern string
	var devices string
	var selectGPU bool

	cmd := &cobra.Command{
		Use:   "generate-tron",
		Short: "Run interactive local Tron vanity wallet generation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(cmd, pattern, devices, selectGPU, local.WalletTron, vanity.ParseTronPattern)
		},
	}

	cmd.Flags().StringVar(&pattern, "pattern", "", "Tron Base58 pattern; must start with pattern:T, use * or ? as wildcards")
	cmd.Flags().StringVar(&devices, "devices", "all", "comma-separated CUDA device ids or all")
	cmd.Flags().BoolVar(&selectGPU, "select-gpu", false, "probe and choose GPU devices before starting")
	return cmd
}

func runGenerate(cmd *cobra.Command, pattern, devices string, selectGPU bool, wallet local.WalletKind, parsePattern func(string) (vanity.Pattern, error)) error {
	if !isTerminal() {
		return cliexit.Printed(cmd, 1, "this command requires an interactive terminal; use the provanity-worker binary for headless EVM runs")
	}

	parsedPattern, err := parsePattern(pattern)
	if err != nil {
		return err
	}
	deviceIDs, requestedSelection, err := local.ParseDeviceIDs(devices)
	if err != nil {
		return err
	}
	if selectGPU {
		requestedSelection = true
	}
	runCtx, stopSignals := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stopSignals()

	deviceLabel := devices
	if requestedSelection {
		probeCtx, cancel := context.WithTimeout(runCtx, 20*time.Second)
		reportedDevices, err := local.ProbeDevices(probeCtx)
		cancel()
		if err != nil {
			return err
		}
		if len(reportedDevices) <= 1 {
			deviceIDs = nil
			deviceLabel = "all"
		} else {
			deviceIDs, err = tui.RunGPUSelect(reportedDevices)
			if err != nil {
				return err
			}
			deviceLabel = formatDeviceIDs(deviceIDs)
		}
	}

	opts := local.Options{
		Wallet:             wallet,
		Pattern:            parsedPattern,
		DeviceIDs:          deviceIDs,
		ProgressIntervalMS: 1000,
	}

	result, err := tui.RunDashboard(runCtx, tui.DashboardOptions{
		Pattern: parsedPattern.String(),
		Devices: deviceLabel,
	}, func(ctx context.Context, emit local.EmitFunc) (local.Result, error) {
		return local.Run(ctx, opts, emit)
	})
	if err != nil {
		if errors.Is(err, context.Canceled) && result.Address == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "generation canceled before any candidate was found")
			return nil
		}
		if result.Address == "" {
			return err
		}
	}

	printFinalResult(cmd.OutOrStdout(), result)
	return nil
}

func printFinalResult(out io.Writer, result local.Result) {
	fmt.Fprintln(out, "")
	if result.Partial {
		fmt.Fprintln(out, "Best candidate so far (search stopped before reaching target)")
	}
	fmt.Fprintf(out, "address: %s\n", result.Address)
	fmt.Fprintf(out, "private key: %s\n", result.PrivateKey)
	fmt.Fprintf(out, "offset: %s\n", result.Offset)
	if result.Score > 0 {
		fmt.Fprintf(out, "score: %s\n", formatScore(result.Score, result.TargetScore))
	}
	fmt.Fprintf(out, "elapsed: %s\n", formatSeconds(result.Stats.ElapsedSec))
	fmt.Fprintf(out, "attempts: %d\n", result.Stats.Attempts)
	if result.Stats.Hashrate > 0 {
		fmt.Fprintf(out, "hashrate: %s%s\n", human.FormatHashrate(result.Stats.Hashrate), hashrateUncertainMarker(result.Stats.HashrateUncertain))
	}
	if result.OutputPath != "" {
		fmt.Fprintf(out, "plaintext result: %s\n", result.OutputPath)
	}
	printVerifyWarning(out)
}

func printVerifyWarning(out io.Writer) {
	subhead := tui.YellowStyle.Render("Confirm the result is correct before depositing real funds.")
	stepNum := func(n int) string {
		return tui.PinkStyle.Render(fmt.Sprintf("%d.", n))
	}
	body := strings.Join([]string{
		subhead,
		"",
		stepNum(1) + " Import the private key into a separate wallet and check",
		"   that it derives the address shown above.",
		stepNum(2) + " Send a small test transaction before transferring any",
		"   meaningful funds.",
	}, "\n")
	panel := tui.Panel("⚠  VERIFY BEFORE USE", body, 0, tui.RedColor())
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, panel)
}

func hashrateUncertainMarker(uncertain bool) string {
	if uncertain {
		return "*"
	}
	return ""
}

func formatScore(score, target int) string {
	if target > 0 {
		return fmt.Sprintf("%d/%d", score, target)
	}
	return fmt.Sprint(score)
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func formatDeviceIDs(ids []int) string {
	if len(ids) == 0 {
		return "all"
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprint(id))
	}
	return strings.Join(parts, ",")
}

func formatSeconds(sec uint64) string {
	duration := time.Duration(sec) * time.Second
	h := int(duration.Hours())
	m := int(duration.Minutes()) % 60
	s := int(duration.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
