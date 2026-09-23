package ethereum

import (
	"testing"
)

func TestParseHexToUint64(t *testing.T) {
	tests := []struct {
		name    string
		hex     string
		want    uint64
		wantErr bool
	}{
		{
			name:    "simple hex",
			hex:     "0xff",
			want:    255,
			wantErr: false,
		},
		{
			name:    "zero",
			hex:     "0x0",
			want:    0,
			wantErr: false,
		},
		{
			name:    "large number",
			hex:     "0xffffffffffffffff",
			want:    18446744073709551615,
			wantErr: false,
		},
		{
			name:    "without 0x prefix",
			hex:     "ff",
			want:    255,
			wantErr: false,
		},
		{
			name:    "decimals value",
			hex:     "0x12", // 18 decimals
			want:    18,
			wantErr: false,
		},
		{
			name:    "six decimals",
			hex:     "0x06", // USDT/USDC decimals
			want:    6,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHexToUint64(tt.hex)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHexToUint64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseHexToUint64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHexToBytes(t *testing.T) {
	tests := []struct {
		name    string
		hex     string
		want    []byte
		wantErr bool
	}{
		{
			name:    "simple hex",
			hex:     "0xff",
			want:    []byte{0xff},
			wantErr: false,
		},
		{
			name:    "address hex",
			hex:     "0xd8da6bf26964af9d7eed9e03e53415d37aa96045",
			want:    []byte{0xd8, 0xda, 0x6b, 0xf2, 0x69, 0x64, 0xaf, 0x9d, 0x7e, 0xed, 0x9e, 0x03, 0xe5, 0x34, 0x15, 0xd3, 0x7a, 0xa9, 0x60, 0x45},
			wantErr: false,
		},
		{
			name:    "odd length",
			hex:     "0xff0",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HexToBytes(tt.hex)
			if (err != nil) != tt.wantErr {
				t.Errorf("HexToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != string(tt.want) {
				t.Errorf("HexToBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}
