package cli

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/woomai/provanity/internal/config"
)

var version = "0.1.0-dev"

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "provanity",
		Short:         "Generate EVM and Tron vanity wallets with CUDA",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractiveWizard(cmd)
		},
	}
	cmd.SetVersionTemplate(versionInfo() + "\n")

	cmd.AddCommand(
		newBenchCommand(),
		newGenerateCommand(),
		newGenerateTronCommand(),
	)

	return cmd
}

func versionInfo() string {
	var b strings.Builder
	fmt.Fprintf(&b, "provanity %s\n", version)
	fmt.Fprintf(&b, "goos/goarch: %s/%s", runtime.GOOS, runtime.GOARCH)
	if paths, err := config.ResolvePaths(); err == nil {
		fmt.Fprintf(&b, "\nconfig dir: %s", paths.ConfigDir)
		fmt.Fprintf(&b, "\ncache dir: %s", paths.CacheDir)
	}
	return b.String()
}
