package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/woomai/provanity/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		var suppressor interface{ SuppressMainPrint() bool }
		if !errors.As(err, &suppressor) || !suppressor.SuppressMainPrint() {
			fmt.Fprintln(os.Stderr, err)
		}
		var exitCoder interface{ ExitCode() int }
		if errors.As(err, &exitCoder) {
			os.Exit(exitCoder.ExitCode())
		}
		os.Exit(1)
	}
}
