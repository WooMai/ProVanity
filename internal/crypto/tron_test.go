package crypto

import "testing"

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
