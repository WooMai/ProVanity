package cuda

import "github.com/woomai/provanity/internal/gpu"

const MaxDevices = 16
const cudaBackendVersion = "provanity-cuda/2"

// PatternLen matches PROVANITY_CUDA_PATTERN_LEN in the CUDA backend ABI.
const PatternLen = 40

// PatternWildcard is the sentinel byte used inside Config.Pattern to mark
// positions that the kernel must ignore. Mirrors
// PROVANITY_CUDA_PATTERN_WILDCARD in provanity_cuda.h.
const PatternWildcard byte = 0xff

type Mode int32

const (
	ModeLeading     Mode = 0
	ModePattern     Mode = 1
	ModeTronPattern Mode = 2
)

type Config struct {
	PublicKeyHex       string
	Mode               Mode
	Contract           bool
	// Pattern encodes the target according to Mode:
	//   ModeLeading:     Pattern[0] holds the target nibble (0..15).
	//   ModePattern:     Pattern[i] holds the target nibble or
	//                    PatternWildcard for unconstrained positions.
	//   ModeTronPattern: Pattern[i] holds the target base58 character or 0
	//                    for unconstrained / unused slots; Pattern[0] is
	//                    implicitly 'T' and not scored.
	Pattern            [PatternLen]byte
	DeviceIDs          []int
	BatchMultiple      uint32
	ProgressIntervalMS uint32
	WorkSize           uint32
	StopScore          byte
}

type EmitFunc func(gpu.Event) bool
