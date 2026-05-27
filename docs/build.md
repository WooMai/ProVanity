# Build from Source

This document describes how to build ProVanity locally from a checkout of this repository. Release binaries are built natively on Windows x64 and Linux x64; cross-compiling is not the recommended path because the CUDA backend is a platform-specific shared library.

## Requirements

- Windows x64 or Linux x64.
- Go 1.25 or newer.
- An NVIDIA GPU and driver supported by ProVanity. See the repository README for the runtime GPU and driver matrix.
- CUDA Toolkit 13.x (for `sm_75` or newer GPUs).
- Windows: Visual Studio 2022 Build Tools with the MSVC C++ toolchain.
- Linux: a C/C++ toolchain, `cgo`, `sh`, and `nvcc` on `PATH` or passed through
  `NVCC`.
- Node.js 18+, Corepack, and pnpm are only needed when packaging npm releases.

The CUDA runtime is linked statically into the ProVanity CUDA backend. A machine that only runs the final release binary needs a compatible NVIDIA driver, not a local CUDA Toolkit install.

## One-command local build

From the repository root:

```bash
go run ./cmd/build
```

By default this command:

1. builds the CUDA backend;
2. copies the backend artifact into `internal/cuda/assets`;
3. runs `go test -count=1 -tags cudaembed ./...`;
4. builds `provanity` and `provanity-worker` with embedded CUDA assets.

The output is written to `./provanity` and `./provanity-worker` on Linux, or `./provanity.exe` and `./provanity-worker.exe` on Windows. The version string is detected with `git describe --tags --always --dirty` unless `-version` is set.

Common options:

```bash
# Build for one GPU architecture. This is much faster during development.
go run ./cmd/build -arch sm_89

# Reuse existing files in internal/cuda/assets.
go run ./cmd/build -skip-cuda

# Skip tests.
go run ./cmd/build -skip-tests

# Build a development binary that loads CUDA libraries from disk.
go run ./cmd/build -no-embed

# Write the CLI binary to a custom path.
go run ./cmd/build -output bin/provanity -version v0.1.1

# Build only the interactive CLI.
go run ./cmd/build -skip-worker
```

## CUDA backend only

The CUDA backend source lives in `internal/cuda/native`. Generated CUDA libraries are not source files; they are build artifacts.

### Windows

The PowerShell script automatically looks for CUDA Toolkit installs under `C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA`. If `-CudaRoot` is not provided, it prefers the newest CUDA 13.x install.

```powershell
# Build for a single Turing-or-newer GPU architecture.
.\scripts\build-cuda-backend.ps1 -Arch sm_89

# Release-style fat binary for all standard targets.
.\scripts\build-cuda-backend.ps1 -Arch all

# Use explicit toolchain paths.
.\scripts\build-cuda-backend.ps1 `
  -CudaRoot "C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.2" `
  -VcVars "C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Auxiliary\Build\vcvars64.bat"
```

The Windows script writes `provanity_cuda_standard.dll` to `.tmp/cuda-backend` and also copies it into `internal/cuda/assets`.

### Linux

The Linux script uses `nvcc` from `PATH` by default. Set `NVCC` when you keep multiple CUDA Toolkit versions installed.

```bash
# Build for the first visible GPU detected by nvidia-smi.
EMBED=1 sh scripts/build-cuda-backend.sh

# Build for one architecture.
ARCH=sm_89 EMBED=1 sh scripts/build-cuda-backend.sh

# Release-style fat binary with CUDA 13.x.
NVCC=/usr/local/cuda-13/bin/nvcc ARCH=all EMBED=1 sh scripts/build-cuda-backend.sh
```

When `EMBED=1` is set, the script copies the generated `.so` into `internal/cuda/assets`. Without `ARCH` or `ARCHS`, it tries to detect the first visible GPU with `nvidia-smi`; if no GPU is visible, it falls back to the full `sm_75+` target list.

Useful CUDA build knobs:

| Windows option | Linux environment variable | Meaning |
| --- | --- | --- |
| `-Arch sm_89` | `ARCH=sm_89` | Build for one architecture. |
| `-Arch all` | `ARCH=all` | Build the fat binary target list. |
| `-Archs ...` | `ARCHS="sm_80 sm_89"` | Build a custom fat binary target list. |
| `-PtxArch compute_120` | `PTX_ARCH=compute_120` | Add a PTX fallback target. |
| `-MaxRegisters 64` | `MAX_REGISTERS=64` | Pass `--maxrregcount` to NVCC. |
| `-PtxasVerbose` | `PTXAS_VERBOSE=1` | Print ptxas resource usage. |
| `-LineInfo` | `LINEINFO=1` | Include line info in the CUDA output. |
| `-PtxasDlcmCa` | `PTXAS_DLCM_CA=1` | Pass the cache-load modifier used for tuning. |
| `-ExtraNvccFlags "..."` | `EXTRA_NVCC_FLAGS="..."` | Append extra NVCC flags. |

## Go binaries only

If CUDA assets are already present in `internal/cuda/assets`, you can build the Go binaries directly:

```bash
go test -count=1 -tags cudaembed ./...
go build -trimpath -tags cudaembed -ldflags "-s -w -X github.com/woomai/provanity/internal/cli.version=v0.1.1" -o bin/provanity ./cmd/provanity
go build -trimpath -tags cudaembed -ldflags "-s -w -X github.com/woomai/provanity/internal/worker.Version=v0.1.1" -o bin/provanity-worker ./cmd/provanity-worker
```

For a development binary that loads the CUDA backend from disk instead of embedding it, omit the `cudaembed` build tag:

```bash
go build -trimpath -o bin/provanity ./cmd/provanity
```

Non-embedded builds search for CUDA libraries next to the executable, under a `cuda` subdirectory, under `.tmp/cuda-backend`, and under `internal/cuda/assets`. You can also point at a specific library:

```powershell
$env:PROVANITY_CUDA_DLL = "C:\path\to\provanity_cuda_standard.dll"
```

```bash
PROVANITY_CUDA_SO=/path/to/provanity_cuda_standard.so ./bin/provanity
```

On Linux, make sure `CGO_ENABLED=1` when building CUDA-enabled binaries. With `CGO_ENABLED=0`, the Linux build uses the stub CUDA implementation.

## npm packages

The npm workspace contains a wrapper package and platform packages:

- `npm/provanity`
- `npm/cli-linux-x64`
- `npm/cli-win32-x64`
- `npm/worker-linux-x64`

Prepare package metadata:

```bash
corepack enable
pnpm install
node scripts/sync-npm-version.mjs v0.1.1
```

Build platform binaries into the package `bin` directories on the matching OS:

```bash
# Linux CLI package.
go run ./cmd/build -skip-worker -output npm/cli-linux-x64/bin/provanity -version v0.1.1

# Linux worker package. Run after CUDA assets have been built.
go build -trimpath -tags cudaembed -ldflags "-s -w -X github.com/woomai/provanity/internal/worker.Version=v0.1.1" -o npm/worker-linux-x64/bin/provanity-worker ./cmd/provanity-worker

# Windows CLI package.
go run ./cmd/build -skip-worker -output npm/cli-win32-x64/bin/provanity.exe -version v0.1.1
```

Then create local package tarballs:

```bash
pnpm pack:npm
```

`npm/provanity` copies the repository README during `prepack` and installs the matching optional platform package during `postinstall`.

## Troubleshooting

- `Missing nvcc`: install the CUDA Toolkit or pass `-CudaRoot` on Windows / set
  `NVCC` on Linux.
- `Missing MSVC vcvars64.bat`: install Visual Studio 2022 Build Tools with the
  C++ workload, or pass `-VcVars`.
- `sm_XX is below provanity's sm_75 minimum`: ProVanity does not support Pascal
  or older GPUs. Use [1inch/profanity2](https://github.com/1inch/profanity2) on
  those cards.
- `CUDA backend version ... does not match`: rebuild the CUDA backend assets
  from the current source tree.
- `embedded CUDA ... is not included in this build`: build with
  `-tags cudaembed` or use `go run ./cmd/build` without `-no-embed`.
