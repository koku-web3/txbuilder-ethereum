package ethereum

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
)

func TestPadAddressTo32Bytes(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "standard 40-char address",
			address: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
			wantErr: false,
		},
		{
			name:    "lowercase address",
			address: "0xd8da6bf26964af9d7eed9e03e53415d37aa96045",
			wantErr: false,
		},
		{
			name:    "without 0x prefix",
			address: "d8da6bf26964af9d7eed9e03e53415d37aa96045",
			wantErr: false,
		},
		{
			name:    "case insensitive",
			address: "0xD8DA6BF26964AF9D7EED9E03E53415D37AA96045",
			wantErr: false,
		},
		{
			name:    "invalid hex chars",
			address: "0xd8da6bf26964af9d7eed9e03e53415d37aaggggg",
			wantErr: true,
		},
		{
			name:    "empty address",
			address: "",
			wantErr: false, // 解码为空字节，等价于零地址
		},
		{
			name:    "odd length hex",
			address: "0xd8da6bf26964af9d7eed9e03e53415d37aa9604",
			wantErr: true,
		},
		{
			name:    "oversized address must error instead of panic",
			address: "0x" + strings.Repeat("ab", 33), // 33 字节 > 32 字节
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := PadAddressTo32Bytes(tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("PadAddressTo32Bytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(result) != 64 {
				t.Errorf("PadAddressTo32Bytes() len = %d, want 64", len(result))
			}
		})
	}
}

func TestPadAddressTo32BytesBytes(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "standard address",
			address: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
			wantErr: false,
		},
		{
			name:    "lowercase",
			address: "0xd8da6bf26964af9d7eed9e03e53415d37aa96045",
			wantErr: false,
		},
		{
			name:    "without 0x prefix",
			address: "d8da6bf26964af9d7eed9e03e53415d37aa96045",
			wantErr: false,
		},
		{
			name:    "invalid hex chars",
			address: "0xd8da6bf26964af9d7eed9e03e53415d37aaggggg",
			wantErr: true,
		},
		{
			name:    "oversized address must error instead of panic",
			address: "0x" + strings.Repeat("ab", 33),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := PadAddressTo32BytesBytes(tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("PadAddressTo32BytesBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(result) != 32 {
				t.Errorf("PadAddressTo32BytesBytes() len = %d, want 32", len(result))
			}

			// 验证前 12 字节是 0（地址是左补零的）
			for i := 0; i < 12; i++ {
				if result[i] != 0 {
					t.Errorf("PadAddressTo32BytesBytes() padding[%d] = %x, want 0", i, result[i])
				}
			}

			// 验证最后 20 字节与非 hex 前缀版本匹配
			expectedAddr := strings.TrimPrefix(tt.address, "0x")
			expectedBytes, _ := hex.DecodeString(expectedAddr)
			for i, b := range expectedBytes {
				if result[12+i] != b {
					t.Errorf("PadAddressTo32BytesBytes() addrBytes[%d] = %x, want %x", i, result[12+i], b)
				}
			}
		})
	}
}

func TestBuildERC20TransferData(t *testing.T) {
	to := "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
	amount := big.NewInt(7800000000000000000)

	data, err := BuildERC20TransferData(to, amount)
	if err != nil {
		t.Fatalf("BuildERC20TransferData() unexpected error: %v", err)
	}

	// 验证长度：4 字节 methodID + 32 字节地址 + 32 字节金额 = 68
	if len(data) != 68 {
		t.Errorf("BuildERC20TransferData() len = %d, want 68", len(data))
	}

	// 验证 methodID
	expectedMethodID := []byte{0xa9, 0x05, 0x9c, 0xbb}
	for i, b := range expectedMethodID {
		if data[i] != b {
			t.Errorf("BuildERC20TransferData() methodID[%d] = %x, want %x", i, data[i], b)
		}
	}

	// 验证前 12 字节是 0（地址是左补零的）
	for i := 0; i < 12; i++ {
		if data[4+i] != 0 {
			t.Errorf("BuildERC20TransferData() padding[%d] = %x, want 0", i, data[4+i])
		}
	}

	// 验证收款地址落在第 4+12 起始的 20 字节
	expectedAddr, _ := hex.DecodeString(strings.ToLower(strings.TrimPrefix(to, "0x")))
	for i, b := range expectedAddr {
		if data[4+12+i] != b {
			t.Errorf("BuildERC20TransferData() addr[%d] = %x, want %x", i, data[4+12+i], b)
		}
	}

	// 验证金额落在末尾 32 字节的大端编码
	wantAmount := make([]byte, 32)
	amtBytes := amount.Bytes()
	copy(wantAmount[32-len(amtBytes):], amtBytes)
	for i, b := range wantAmount {
		if data[36+i] != b {
			t.Errorf("BuildERC20TransferData() amount[%d] = %x, want %x", i, data[36+i], b)
		}
	}
}

func TestBuildERC20TransferData_InvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		to      string
		amount  *big.Int
		wantErr bool
	}{
		{
			name:    "invalid recipient hex",
			to:      "0xzzzz",
			amount:  big.NewInt(1),
			wantErr: true,
		},
		{
			name:    "oversized recipient must error instead of panic",
			to:      "0x" + strings.Repeat("ab", 33),
			amount:  big.NewInt(1),
			wantErr: true,
		},
		{
			name:    "amount exceeding uint256 must error instead of panic",
			to:      "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
			amount:  new(big.Int).Lsh(big.NewInt(1), 256),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildERC20TransferData(tt.to, tt.amount); (err != nil) != tt.wantErr {
				t.Errorf("BuildERC20TransferData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPadAmountTo32Bytes(t *testing.T) {
	tests := []struct {
		name    string
		amount  *big.Int
		wantErr bool
	}{
		{name: "zero", amount: big.NewInt(0), wantErr: false},
		{name: "small", amount: big.NewInt(1), wantErr: false},
		{
			name:   "max uint256",
			amount: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)),
			// max uint256 正好 32 字节，允许通过
			wantErr: false,
		},
		{
			name:    "uint256 overflow must error instead of panic",
			amount:  new(big.Int).Lsh(big.NewInt(1), 256),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PadAmountTo32Bytes(tt.amount)
			if (err != nil) != tt.wantErr {
				t.Errorf("PadAmountTo32Bytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(got) != 32 {
				t.Errorf("PadAmountTo32Bytes() len = %d, want 32", len(got))
			}
			if new(big.Int).SetBytes(got).Cmp(tt.amount) != 0 {
				t.Errorf("PadAmountTo32Bytes() round-trip = %s, want %s", new(big.Int).SetBytes(got), tt.amount)
			}
		})
	}
}
