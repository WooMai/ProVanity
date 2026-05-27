#!/usr/bin/env sh
set -eu

SOURCE_DIR="${SOURCE_DIR:-$(CDPATH= cd -- "$(dirname -- "$0")/../internal/cuda/native" && pwd)}"
BUILD_DIR="${BUILD_DIR:-$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)/.tmp/cuda-backend}"
EMBED_DIR="${EMBED_DIR:-$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)/internal/cuda/assets}"
NVCC="${NVCC:-nvcc}"
ARCH="${ARCH:-}"
ARCHS="${ARCHS:-}"
PTX_ARCH="${PTX_ARCH:-}"
DEFAULT_ARCHS="sm_75 sm_80 sm_86 sm_87 sm_88 sm_89 sm_90 sm_100 sm_103 sm_110 sm_120 sm_121"
DEFAULT_PTX_ARCH="compute_120"
MAX_REGISTERS="${MAX_REGISTERS:-0}"
PTXAS_VERBOSE="${PTXAS_VERBOSE:-0}"
LINEINFO="${LINEINFO:-0}"
PTXAS_DLCM_CA="${PTXAS_DLCM_CA:-0}"
EXTRA_NVCC_FLAGS="${EXTRA_NVCC_FLAGS:-}"
EMBED="${EMBED:-0}"
OUTPUT_NAME="${OUTPUT_NAME:-provanity_cuda_standard.so}"

source_file="$SOURCE_DIR/backend.cu"
output_file="$BUILD_DIR/$OUTPUT_NAME"

if [ ! -f "$source_file" ]; then
  echo "Missing CUDA source at $source_file" >&2
  exit 1
fi
if ! command -v "$NVCC" >/dev/null 2>&1; then
  echo "Missing nvcc. Install CUDA Toolkit or set NVCC." >&2
  exit 1
fi
if [ "$MAX_REGISTERS" -lt 0 ]; then
  echo "MAX_REGISTERS must be 0 or greater." >&2
  exit 1
fi

detect_cuda_arch() {
  if ! command -v nvidia-smi >/dev/null 2>&1; then
    return 1
  fi
  compute_cap=$(nvidia-smi --query-gpu=compute_cap --format=csv,noheader,nounits 2>/dev/null | sed -n '1{s/[[:space:]]//g;p;}')
  case "$compute_cap" in
    [0-9]*.[0-9]*)
      major=${compute_cap%%.*}
      minor=${compute_cap#*.}
      minor=${minor%%[!0-9]*}
      if [ -n "$major" ] && [ -n "$minor" ]; then
        printf 'sm_%s%s\n' "$major" "$minor"
        return 0
      fi
      ;;
  esac
  return 1
}

require_supported_arch() {
  arch="$1"
  if [ "$arch" = "all" ]; then
    return 0
  fi
  case "$arch" in
    sm_50|sm_52|sm_53|sm_60|sm_61|sm_62|sm_70|sm_72)
      echo "Error: $arch is below provanity's sm_75 minimum. Pascal/Volta and older are not supported." >&2
      exit 1
      ;;
  esac
}

set -- -std=c++17 -O3 -Xcompiler -fPIC -shared -cudart=static
if [ "$ARCH" = "all" ]; then
  ARCH=""
  ARCHS="$DEFAULT_ARCHS"
  PTX_ARCH="${PTX_ARCH:-$DEFAULT_PTX_ARCH}"
elif [ -n "$ARCH" ]; then
  require_supported_arch "$ARCH"
  set -- "$@" "-arch=$ARCH"
else
  if [ -z "$ARCHS" ]; then
    if detected_arch=$(detect_cuda_arch); then
      ARCH="$detected_arch"
    else
      ARCHS="$DEFAULT_ARCHS"
      PTX_ARCH="${PTX_ARCH:-$DEFAULT_PTX_ARCH}"
    fi
  fi
  if [ -n "$ARCH" ]; then
    require_supported_arch "$ARCH"
    set -- "$@" "-arch=$ARCH"
  fi
fi
if [ -z "$ARCH" ]; then
  for sm_arch in $ARCHS; do
    require_supported_arch "$sm_arch"
    compute_arch="compute_${sm_arch#sm_}"
    set -- "$@" -gencode "arch=$compute_arch,code=$sm_arch"
  done
  if [ -n "$PTX_ARCH" ]; then
    set -- "$@" -gencode "arch=$PTX_ARCH,code=$PTX_ARCH"
  fi
fi
if [ "$MAX_REGISTERS" -gt 0 ]; then
  set -- "$@" "--maxrregcount=$MAX_REGISTERS"
fi
if [ "$PTXAS_VERBOSE" = "1" ]; then
  set -- "$@" -Xptxas -v
fi
if [ "$LINEINFO" = "1" ]; then
  set -- "$@" -lineinfo
fi
if [ "$PTXAS_DLCM_CA" = "1" ]; then
  set -- "$@" -Xptxas -O3,-dlcm=ca
fi
if [ -n "$EXTRA_NVCC_FLAGS" ]; then
  # shellcheck disable=SC2086
  set -- "$@" $EXTRA_NVCC_FLAGS
fi

mkdir -p "$BUILD_DIR"
"$NVCC" "$@" -o "$output_file" "$source_file"

if [ "$EMBED" = "1" ]; then
  mkdir -p "$EMBED_DIR"
  cp "$output_file" "$EMBED_DIR/$OUTPUT_NAME"
  echo "Updated embedded CUDA shared library asset in $EMBED_DIR"
fi

echo "Built $output_file"
