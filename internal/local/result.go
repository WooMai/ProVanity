package local

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	provanitycrypto "github.com/woomai/provanity/internal/crypto"
	"github.com/woomai/provanity/internal/gpu"
)

func finalizeCUDAResult(keypair provanitycrypto.KeyPair, opts Options, found gpu.FoundEvent, progress gpu.ProgressEvent) (Result, error) {
	offset, err := hex.DecodeString(found.Offset)
	if err != nil {
		return Result{}, fmt.Errorf("decode cuda offset: %w", err)
	}
	address, err := hex.DecodeString(found.Address)
	if err != nil {
		return Result{}, fmt.Errorf("decode cuda address: %w", err)
	}
	if len(keypair.PrivateKey) == 0 {
		return remoteCUDAResult(keypair, opts, found, progress, offset, address)
	}
	finalized, err := provanitycrypto.Finalize(keypair.PrivateKey, offset, address)
	if err != nil {
		return Result{}, fmt.Errorf("finalize split key: %w", err)
	}

	addressText, err := displayAddress(opts, finalized.Address)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Address:     addressText,
		PrivateKey:  "0x" + hex.EncodeToString(finalized.PrivateKey),
		PublicKey:   "0x" + hex.EncodeToString(finalized.PublicKey),
		Offset:      "0x" + found.Offset,
		Pattern:     opts.Pattern.String(),
		Score:       scoreAddress(opts, addressText, hex.EncodeToString(finalized.Address)),
		TargetScore: opts.Pattern.TargetScore(),
		ScoreBase:   scoreBase(opts),
		Stats:       resultStats(found, progress),
	}, nil
}

func remoteCUDAResult(keypair provanitycrypto.KeyPair, opts Options, found gpu.FoundEvent, progress gpu.ProgressEvent, offset, address []byte) (Result, error) {
	if len(offset) != provanitycrypto.PrivateKeySize {
		return Result{}, fmt.Errorf("offset must be %d bytes", provanitycrypto.PrivateKeySize)
	}
	if len(address) != provanitycrypto.AddressSize {
		return Result{}, fmt.Errorf("address must be %d bytes", provanitycrypto.AddressSize)
	}

	publicKey := ""
	if len(keypair.PublicKey) > 0 {
		publicKey = "0x" + hex.EncodeToString(keypair.PublicKey)
	}

	addressText, err := displayAddress(opts, address)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Address:     addressText,
		PublicKey:   publicKey,
		Offset:      "0x" + hex.EncodeToString(offset),
		Pattern:     opts.Pattern.String(),
		Score:       scoreAddress(opts, addressText, hex.EncodeToString(address)),
		TargetScore: opts.Pattern.TargetScore(),
		ScoreBase:   scoreBase(opts),
		Stats:       resultStats(found, progress),
	}, nil
}

func resultStats(found gpu.FoundEvent, progress gpu.ProgressEvent) Stats {
	hashrate := progress.Hashrate
	if hashrate == 0 && found.ElapsedSec > 0 {
		hashrate = found.Attempts / found.ElapsedSec
	}
	return Stats{
		ElapsedSec:        found.ElapsedSec,
		Attempts:          found.Attempts,
		Hashrate:          hashrate,
		HashrateUncertain: progress.HashrateUncertain,
	}
}

func writeResult(path string, result Result) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create result directory: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode result: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write plaintext result: %w", err)
	}
	return nil
}

func candidateAddress(opts Options, addressHex string) string {
	if opts.Wallet != WalletTron {
		return "0x" + strings.TrimPrefix(addressHex, "0x")
	}
	address, err := hex.DecodeString(strings.TrimPrefix(addressHex, "0x"))
	if err != nil {
		return addressHex
	}
	tronAddress, err := provanitycrypto.TronAddressFromEVMAddress(address)
	if err != nil {
		return addressHex
	}
	return tronAddress
}

func displayAddress(opts Options, address []byte) (string, error) {
	if opts.Wallet == WalletTron {
		return provanitycrypto.TronAddressFromEVMAddress(address)
	}
	return "0x" + hex.EncodeToString(address), nil
}

func scoreAddress(opts Options, display, evmHex string) int {
	if opts.Wallet == WalletTron {
		return opts.Pattern.ScoreAddress(display)
	}
	return opts.Pattern.ScoreAddressHex(evmHex)
}

func matchesAddress(opts Options, display, evmHex string) bool {
	if opts.Wallet == WalletTron {
		return opts.Pattern.MatchesAddress(display)
	}
	return opts.Pattern.MatchesAddressHex(evmHex)
}

func scoreBase(opts Options) int {
	if opts.Wallet == WalletTron {
		return 58
	}
	return 16
}
