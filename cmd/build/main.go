// Command build is the single-entry build wrapper for ProVanity.
//
//	go run ./cmd/build                 # CUDA backend + go test + go build, embed enabled
//	go run ./cmd/build -skip-tests     # skip the test step
//	go run ./cmd/build -skip-cuda      # reuse the CUDA artifact already in internal/cuda/assets
//	go run ./cmd/build -no-embed       # build a dev binary that loads the CUDA backend at runtime
//	go run ./cmd/build -arch sm_89     # single-arch CUDA build for fast dev iteration
//	go run ./cmd/build -output bin/provanity -version v1.2.3
//	go run ./cmd/build -skip-worker    # only build the interactive CLI
//
// Two binaries are produced by default: provanity (interactive CLI) and
// provanity-worker (standalone headless GPU worker). The worker binary is
// placed next to -output (provanity-worker[.exe] in the same directory).
//
// The underlying scripts/build-cuda-backend.{ps1,sh} and `go {test,build}`
// commands still work; this tool just chains them with consistent defaults.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	versionSymbol       = "github.com/woomai/provanity/internal/cli.version"
	workerVersionSymbol = "github.com/woomai/provanity/internal/worker.Version"
)

type options struct {
	skipTests  bool
	skipCUDA   bool
	skipWorker bool
	noEmbed    bool
	arch       string
	output     string
	version    string
}

func main() {
	var opts options
	flag.BoolVar(&opts.skipTests, "skip-tests", false, "skip the go test step")
	flag.BoolVar(&opts.skipCUDA, "skip-cuda", false, "skip the CUDA backend build and reuse internal/cuda/assets/*")
	flag.BoolVar(&opts.skipWorker, "skip-worker", false, "skip building the standalone provanity-worker binary")
	flag.BoolVar(&opts.noEmbed, "no-embed", false, "build without the cudaembed tag (dev binary loads the CUDA backend at runtime)")
	flag.StringVar(&opts.arch, "arch", "all", "CUDA arch: 'all' for release fat binaries, or a single sm_XX like sm_89 for fast dev iteration")
	flag.StringVar(&opts.output, "output", "", "provanity binary path (default: ./provanity[.exe]); provanity-worker is placed next to it")
	flag.StringVar(&opts.version, "version", "", "version string injected at link time (default: git describe --tags --always --dirty)")
	flag.Parse()

	repoRoot, err := findRepoRoot()
	if err != nil {
		die(err)
	}

	if opts.version == "" {
		opts.version = detectVersion(repoRoot)
	}
	if opts.output == "" {
		opts.output = defaultOutput(repoRoot)
	}

	if opts.skipCUDA {
		section("skip CUDA backend (reusing existing artifact)")
	} else {
		section("build CUDA backend")
		if err := buildCUDA(repoRoot, opts.arch); err != nil {
			die(fmt.Errorf("CUDA backend build: %w", err))
		}
	}

	if opts.skipTests {
		section("skip go test")
	} else {
		section("go test")
		if err := runTests(repoRoot, !opts.noEmbed); err != nil {
			die(fmt.Errorf("go test: %w", err))
		}
	}

	section(fmt.Sprintf("go build -> %s", opts.output))
	if err := buildBinary(repoRoot, opts, "./cmd/provanity", opts.output, versionSymbol); err != nil {
		die(fmt.Errorf("go build provanity: %w", err))
	}

	workerOutput := ""
	if !opts.skipWorker {
		workerOutput = workerOutputFor(opts.output)
		section(fmt.Sprintf("go build -> %s", workerOutput))
		if err := buildBinary(repoRoot, opts, "./cmd/provanity-worker", workerOutput, workerVersionSymbol); err != nil {
			die(fmt.Errorf("go build provanity-worker: %w", err))
		}
	}

	fmt.Println()
	fmt.Printf("built ProVanity %s -> %s\n", opts.version, opts.output)
	if workerOutput != "" {
		fmt.Printf("built provanity-worker %s -> %s\n", opts.version, workerOutput)
	}
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", cwd)
		}
		dir = parent
	}
}

func detectVersion(repoRoot string) string {
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "0.0.0-dev"
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "0.0.0-dev"
	}
	return v
}

func buildCUDA(repoRoot, arch string) error {
	switch runtime.GOOS {
	case "windows":
		script := filepath.Join(repoRoot, "scripts", "build-cuda-backend.ps1")
		args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script}
		if arch != "" {
			args = append(args, "-Arch", arch)
		}
		return run(repoRoot, nil, "powershell", args...)
	case "linux":
		// EMBED=1 keeps Linux behavior consistent with the Windows script,
		// which always copies the artifact into internal/cuda/assets.
		script := filepath.Join(repoRoot, "scripts", "build-cuda-backend.sh")
		env := []string{"EMBED=1"}
		if arch != "" {
			env = append(env, "ARCH="+arch)
		}
		return run(repoRoot, env, "sh", script)
	default:
		return fmt.Errorf("unsupported OS %q (only windows and linux ship a CUDA backend)", runtime.GOOS)
	}
}

func runTests(repoRoot string, embed bool) error {
	args := []string{"test", "-count=1"}
	if embed {
		args = append(args, "-tags", "cudaembed")
	}
	args = append(args, "./...")
	return run(repoRoot, nil, "go", args...)
}

func buildBinary(repoRoot string, opts options, pkg, output, verSymbol string) error {
	args := []string{"build", "-trimpath"}
	if !opts.noEmbed {
		args = append(args, "-tags", "cudaembed")
	}
	ldflags := "-s -w"
	if opts.version != "" {
		ldflags += " -X " + verSymbol + "=" + opts.version
	}
	args = append(args, "-ldflags", ldflags, "-o", output, pkg)
	return run(repoRoot, nil, "go", args...)
}

func defaultOutput(repoRoot string) string {
	name := "provanity"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(repoRoot, name)
}

// workerOutputFor derives the provanity-worker binary path from the main
// provanity output path: it lives in the same directory, with name
// provanity-worker plus the platform extension.
func workerOutputFor(mainOutput string) string {
	dir := filepath.Dir(mainOutput)
	name := "provanity-worker"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}

func run(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	fmt.Printf("    %s %s\n", name, strings.Join(args, " "))
	return cmd.Run()
}

func section(title string) {
	fmt.Println()
	fmt.Println("==>", title)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "build:", err)
	os.Exit(1)
}
