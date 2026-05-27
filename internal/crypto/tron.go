package crypto

import (
	"crypto/sha256"
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
	return base58CheckEncode(payload), nil
}

func base58CheckEncode(payload []byte) string {
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])

	checked := make([]byte, 0, len(payload)+4)
	checked = append(checked, payload...)
	checked = append(checked, second[:4]...)
	return base58Encode(checked)
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
