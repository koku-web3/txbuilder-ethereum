package ethereum

import (
	"testing"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{
			name:    "valid lowercase hex",
			address: "0xfb6919d4b3f2f1c5b5b5b5b5b5b5b5b5b5b5b5b5",
			want:    true,
		},
		{
			name:    "valid uppercase hex",
			address: "0xFB6919D4B3F2F1C5B5B5B5B5B5B5B5B5B5B5B5B5",
			want:    true,
		},
		{
			name:    "valid mixed case EIP-55",
			address: "0xFb6919d4b3F2f1C5b5b5b5b5b5b5b5b5b5b5b5b5",
			want:    true,
		},
		{
			name:    "valid vitalik address",
			address: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
			want:    true,
		},
		{
			name:    "invalid too short",
			address: "0x1234",
			want:    false,
		},
		{
			name:    "invalid contains Z",
			address: "0x1234567890abcdef1234567890abcdef1234567Z",
			want:    false,
		},
		{
			name:    "invalid no 0x prefix",
			address: "fb6919d4b3f2f1c5b5b5b5b5b5b5b5b5b5b5b5b5",
			want:    false,
		},
		{
			name:    "invalid empty",
			address: "",
			want:    false,
		},
		{
			name:    "invalid only 0x",
			address: "0x",
			want:    false,
		},
		{
			name:    "invalid 0X uppercase prefix",
			address: "0Xfb6919d4b3f2f1c5b5b5b5b5b5b5b5b5b5b5b5b5",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateAddress(tt.address); got != tt.want {
				t.Errorf("ValidateAddress(%q) = %v, want %v", tt.address, got, tt.want)
			}
		})
	}
}

func TestValidateContractAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{
			name:    "valid contract address",
			address: "0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984",
			want:    true,
		},
		{
			name:    "invalid contract address",
			address: "0x1234",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateContractAddress(tt.address); got != tt.want {
				t.Errorf("ValidateContractAddress(%q) = %v, want %v", tt.address, got, tt.want)
			}
		})
	}
}

func TestIsPureNumber(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{
			name: "valid number",
			s:    "1000000",
			want: true,
		},
		{
			name: "valid large number",
			s:    "1000000000000000000",
			want: true,
		},
		{
			name: "invalid with decimal",
			s:    "100.50",
			want: false,
		},
		{
			name: "invalid with letter",
			s:    "100abc",
			want: false,
		},
		{
			name: "invalid empty",
			s:    "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPureNumber(tt.s); got != tt.want {
				t.Errorf("IsPureNumber(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestNormalizeHex(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
	}{
		{
			name:    "0x prefix lowercase",
			address: "0xfb6919d4b3f2f1c5b5b5b5b5b5b5b5b5b5b5b5b5",
			want:    "fb6919d4b3f2f1c5b5b5b5b5b5b5b5b5b5b5b5b5",
		},
		{
			name:    "0x prefix uppercase",
			address: "0xFB6919D4B3F2F1C5B5B5B5B5B5B5B5B5B5B5B5B5",
			want:    "fb6919d4b3f2f1c5b5b5b5b5b5b5b5b5b5b5b5b5",
		},
		{
			name:    "40 char hex lowercase",
			address: "fb6919d4b3f2f1c5b5b5b5b5b5b5b5b5b5b5b5b5",
			want:    "fb6919d4b3f2f1c5b5b5b5b5b5b5b5b5b5b5b5b5",
		},
		{
			name:    "40 char hex uppercase",
			address: "FB6919D4B3F2F1C5B5B5B5B5B5B5B5B5B5B5B5B5",
			want:    "fb6919d4b3f2f1c5b5b5b5b5b5b5b5b5b5b5b5b5",
		},
		{
			name:    "invalid format",
			address: "invalid",
			want:    "",
		},
		{
			name:    "empty",
			address: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeHex(tt.address); got != tt.want {
				t.Errorf("NormalizeHex(%q) = %v, want %v", tt.address, got, tt.want)
			}
		})
	}
}

func TestHexToAddress(t *testing.T) {
	tests := []struct {
		name      string
		hex       string
		wantBytes []byte
		wantErr   bool
	}{
		{
			name:      "valid address",
			hex:       "0xfb6919d4b3f2f1c5b5b5b5b5b5b5b5b5b5b5b5b5",
			wantBytes: []byte{0xfb, 0x69, 0x19, 0xd4, 0xb3, 0xf2, 0xf1, 0xc5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5},
			wantErr:   false,
		},
		{
			name:      "invalid length",
			hex:       "0x1234",
			wantBytes: nil,
			wantErr:   true,
		},
		{
			name:      "invalid hex chars",
			hex:       "0xfb6919d4b3f2f1c5b5b5b5b5b5b5b5b5b5b5b5g5",
			wantBytes: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HexToAddress(tt.hex)
			if (err != nil) != tt.wantErr {
				t.Errorf("HexToAddress(%q) error = %v, wantErr %v", tt.hex, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.wantBytes) {
					t.Errorf("HexToAddress(%q) = %x, want %x", tt.hex, got, tt.wantBytes)
					return
				}
				for i := range got {
					if got[i] != tt.wantBytes[i] {
						t.Errorf("HexToAddress(%q) = %x, want %x", tt.hex, got, tt.wantBytes)
						return
					}
				}
			}
		})
	}
}

func TestAddressToHex(t *testing.T) {
	tests := []struct {
		name    string
		address []byte
		want    string
	}{
		{
			name:    "valid 20 bytes",
			address: []byte{0xfb, 0x69, 0x19, 0xd4, 0xb3, 0xf2, 0xf1, 0xc5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5, 0xb5},
			want:    "0xfb6919d4b3f2f1c5b5b5b5b5b5b5b5b5b5b5b5b5",
		},
		{
			name:    "invalid length",
			address: []byte{0xfb, 0x69},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AddressToHex(tt.address); got != tt.want {
				t.Errorf("AddressToHex(%x) = %v, want %v", tt.address, got, tt.want)
			}
		})
	}
}
