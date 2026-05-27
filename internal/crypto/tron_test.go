package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestTronAddressFromZeroEVMAddress(t *testing.T) {
	address := make([]byte, AddressSize)
	got, err := TronAddressFromEVMAddress(address)
	if err != nil {
		t.Fatalf("TronAddressFromEVMAddress() error = %v", err)
	}
	if got != "T9yD14Nj9j7xAB4dbGeiX9h8unkKHxuWwb" {
		t.Fatalf("Tron address = %s", got)
	}
}

func TestTronAddressRejectsWrongSize(t *testing.T) {
	if _, err := TronAddressFromEVMAddress([]byte{1, 2, 3}); err == nil {
		t.Fatal("TronAddressFromEVMAddress() succeeded, want error")
	}
}

func TestTronAddressRoundTrip(t *testing.T) {
	for range 256 {
		address := make([]byte, AddressSize)
		if _, err := rand.Read(address); err != nil {
			t.Fatalf("rand.Read() error = %v", err)
		}
		encoded, err := TronAddressFromEVMAddress(address)
		if err != nil {
			t.Fatalf("TronAddressFromEVMAddress() error = %v", err)
		}
		payload, err := base58CheckDecode(encoded)
		if err != nil {
			t.Fatalf("base58CheckDecode(%q) error = %v", encoded, err)
		}
		if payload[0] != tronVersion {
			t.Fatalf("version byte = 0x%02x, want 0x%02x", payload[0], tronVersion)
		}
		if !bytes.Equal(payload[1:], address) {
			t.Fatalf("round-trip mismatch\nwant %x\n got %x", address, payload[1:])
		}
	}
}

func TestBase58CheckDecodeRejectsCorruption(t *testing.T) {
	encoded := "T9yD14Nj9j7xAB4dbGeiX9h8unkKHxuWwb"

	if _, err := base58CheckDecode(encoded[:len(encoded)-1] + "c"); err == nil {
		t.Fatal("base58CheckDecode() accepted a flipped checksum character")
	}
	if _, err := base58CheckDecode(encoded + "0"); err == nil {
		t.Fatal("base58CheckDecode() accepted an invalid base58 character")
	}
}
