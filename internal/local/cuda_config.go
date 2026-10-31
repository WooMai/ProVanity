package local

import (
	"encoding/hex"
	"fmt"

	provanitycrypto "github.com/woomai/provanity/internal/crypto"
	"github.com/woomai/provanity/internal/cuda"
	"github.com/woomai/provanity/internal/vanity"
)

func cudaConfig(opts Options, keypair provanitycrypto.KeyPair) (cuda.Config, error) {
	opts = withCompleteCUDAParams(opts)
	cfg := cuda.Config{
		PublicKeyHex:       hex.EncodeToString(keypair.PublicKey),
		DeviceIDs:          append([]int(nil), opts.DeviceIDs...),
		BatchMultiple:      uint32(opts.BatchMultiple),
		WorkSize:           uint32(opts.WorkSize),
		ProgressIntervalMS: uint32(opts.ProgressIntervalMS),
	}
	switch {
	case opts.Wallet == WalletTron && opts.Pattern.Kind == vanity.PatternTronPrefix:
		// Each prefix length maps to a nested [lo,hi] address interval; the
		// kernel reports the deepest interval that contains a candidate as the
		// matched-character count (graded progress, no base58/SHA per candidate).
		los, his, ok := provanitycrypto.TronPrefixLadder(opts.Pattern.Value)
		if !ok || len(los) == 0 {
			return cuda.Config{}, fmt.Errorf("Tron prefix %q is not a reachable address prefix", opts.Pattern.Value)
		}
		if len(los) > cuda.TronMaxPrefixLevels {
			return cuda.Config{}, fmt.Errorf("Tron prefix has %d levels, exceeds %d", len(los), cuda.TronMaxPrefixLevels)
		}
		cfg.Mode = cuda.ModeTronPrefix
		cfg.StopScore = byte(opts.Pattern.TargetScore())
		cfg.TronPrefixLevels = byte(len(los))
		for j := range los {
			off := j * 2 * provanitycrypto.AddressSize
			copy(cfg.TronPrefixLadder[off:off+provanitycrypto.AddressSize], los[j][:])
			copy(cfg.TronPrefixLadder[off+provanitycrypto.AddressSize:off+2*provanitycrypto.AddressSize], his[j][:])
		}
	case opts.Wallet == WalletTron && opts.Pattern.Kind == vanity.PatternTronSuffix:
		// The suffix is checksum-dependent, so the kernel computes the real
		// base58check tail per candidate. We pass the target digit values
		// (index 0 = last character) and the modulus 58^N used to extract the
		// address tail via value mod 58^N.
		value := opts.Pattern.Value
		n := len(value)
		if n == 0 || n > cuda.TronMaxSuffixLen {
			return cuda.Config{}, fmt.Errorf("Tron suffix length %d is out of range (1..%d)", n, cuda.TronMaxSuffixLen)
		}
		cfg.Mode = cuda.ModeTronSuffix
		cfg.StopScore = byte(opts.Pattern.TargetScore())
		cfg.TronSuffixLen = byte(n)
		mod := uint64(1)
		for i := 0; i < n; i++ {
			idx := provanitycrypto.Base58Index(value[n-1-i])
			if idx < 0 {
				return cuda.Config{}, fmt.Errorf("Tron suffix has invalid base58 character %q", value[n-1-i])
			}
			cfg.TronSuffixDigits[i] = byte(idx)
			mod *= 58
		}
		cfg.TronSuffixMod = mod
	case opts.Wallet == WalletTron:
		return cuda.Config{}, fmt.Errorf("Tron only supports prefix or suffix mode")
	case opts.Pattern.Kind == vanity.PatternPattern:
		cfg.Mode = cuda.ModePattern
		// Each nibble position is either a target value 0..15 or
		// cuda.PatternWildcard when the user wrote X / x / * / ?.
		for i := range cfg.Pattern {
			cfg.Pattern[i] = cuda.PatternWildcard
		}
		value := opts.Pattern.Value
		limit := len(value)
		if limit > cuda.PatternLen {
			limit = cuda.PatternLen
		}
		for i := 0; i < limit; i++ {
			cfg.Pattern[i] = patternNibble(value[i])
		}
		cfg.StopScore = byte(opts.Pattern.TargetScore())
	case opts.Pattern.Kind == vanity.PatternLeading:
		cfg.Mode = cuda.ModeLeading
		cfg.Pattern[0] = patternNibble(opts.Pattern.Value[0])
		cfg.StopScore = byte(opts.Pattern.TargetScore())
	default:
		return cuda.Config{}, fmt.Errorf("unsupported CUDA pattern kind %q", opts.Pattern.Kind)
	}
	return cfg, nil
}

// patternNibble maps a parsed pattern character to a hex nibble or the CUDA wildcard.
func patternNibble(ch byte) byte {
	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0'
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10
	case ch >= 'A' && ch <= 'F':
		return ch - 'A' + 10
	default:
		return cuda.PatternWildcard
	}
}
