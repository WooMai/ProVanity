package crypto

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
)

const tronVersion byte = 0x41

var base58Alphabet = []byte("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz")

func TronAddressFromEVMAddress(address []byte) (string, error) {
	if len(address) != AddressSize {
		return "", fmt.Errorf("address must be %d bytes", AddressSize)
	}
	payload := make([]byte, 1+AddressSize)
	payload[0] = tronVersion
	copy(payload[1:], address)
	encoded := base58CheckEncode(payload)
	// Decode our own output and confirm it round-trips back to the same
	// 20-byte address. The hand-rolled base58check encoder is the last step
	// before the string the user copies, and the secp256k1/keccak round-trip
	// in Finalize only validates the EVM address, not this encoding — so a
	// silent bug here would hand the user an address they don't control.
	if err := verifyTronAddress(encoded, address); err != nil {
		return "", err
	}
	return encoded, nil
}

func verifyTronAddress(encoded string, address []byte) error {
	payload, err := base58CheckDecode(encoded)
	if err != nil {
		return fmt.Errorf("verify tron address: %w", err)
	}
	if len(payload) != 1+AddressSize {
		return fmt.Errorf("verify tron address: payload is %d bytes, want %d", len(payload), 1+AddressSize)
	}
	if payload[0] != tronVersion {
		return fmt.Errorf("verify tron address: version byte 0x%02x, want 0x%02x", payload[0], tronVersion)
	}
	if !equalBytes(payload[1:], address) {
		return errors.New("verify tron address: decoded address does not match")
	}
	return nil
}

func base58CheckEncode(payload []byte) string {
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])

	checked := make([]byte, 0, len(payload)+4)
	checked = append(checked, payload...)
	checked = append(checked, second[:4]...)
	return base58Encode(checked)
}

func base58CheckDecode(encoded string) ([]byte, error) {
	decoded, err := base58Decode(encoded)
	if err != nil {
		return nil, err
	}
	if len(decoded) < 4 {
		return nil, fmt.Errorf("base58check payload is %d bytes, want at least 4", len(decoded))
	}
	payload := decoded[:len(decoded)-4]
	checksum := decoded[len(decoded)-4:]
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	if !bytes.Equal(checksum, second[:4]) {
		return nil, errors.New("base58check checksum mismatch")
	}
	return payload, nil
}

func base58Decode(encoded string) ([]byte, error) {
	zeroes := 0
	for zeroes < len(encoded) && encoded[zeroes] == base58Alphabet[0] {
		zeroes++
	}

	// Accumulate the base58 digits into a little-endian base256 value.
	decoded := make([]byte, 0, len(encoded))
	for i := zeroes; i < len(encoded); i++ {
		value := base58Index(encoded[i])
		if value < 0 {
			return nil, fmt.Errorf("invalid base58 character %q", encoded[i])
		}
		carry := value
		for j := range decoded {
			carry += int(decoded[j]) * 58
			decoded[j] = byte(carry)
			carry >>= 8
		}
		for carry > 0 {
			decoded = append(decoded, byte(carry))
			carry >>= 8
		}
	}

	for i, j := 0, len(decoded)-1; i < j; i, j = i+1, j-1 {
		decoded[i], decoded[j] = decoded[j], decoded[i]
	}

	result := make([]byte, zeroes+len(decoded))
	copy(result[zeroes:], decoded)
	return result, nil
}

func base58Index(ch byte) int {
	for i, a := range base58Alphabet {
		if a == ch {
			return i
		}
	}
	return -1
}

func base58Encode(value []byte) string {
	zeroes := 0
	for zeroes < len(value) && value[zeroes] == 0 {
		zeroes++
	}

	input := append([]byte(nil), value...)
	encoded := make([]byte, 0, len(value)*138/100+1)
	for start := zeroes; start < len(input); {
		remainder := 0
		for i := start; i < len(input); i++ {
			acc := remainder*256 + int(input[i])
			input[i] = byte(acc / 58)
			remainder = acc % 58
		}
		encoded = append(encoded, base58Alphabet[remainder])
		for start < len(input) && input[start] == 0 {
			start++
		}
	}

	for i := 0; i < zeroes; i++ {
		encoded = append(encoded, base58Alphabet[0])
	}
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}
	return string(encoded)
}
