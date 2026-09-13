package service

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/config"
)

func TestVerifyAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		traceID   string
		wantValid bool
		wantErr   bool
	}{
		{
			name:      "valid address",
			address:   "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
			traceID:   "test-trace-id",
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "invalid address - too short",
			address:   "0x1234",
			traceID:   "test-trace-id",
			wantValid: false,
			wantErr:   false,
		},
		{
			name:      "missing trace_id",
			address:   "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
			traceID:   "",
			wantValid: false,
			wantErr:   true,
		},
		{
			name:      "missing address",
			address:   "",
			traceID:   "test-trace-id",
			wantValid: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			config.SetConfigForTest(cfg)

			svc := &TxBuilderService{}

			req := &grpc.VerifyAddressRequest{
				TraceId: tt.traceID,
				Address: tt.address,
			}

			resp, err := svc.VerifyAddress(context.Background(), req)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && resp.IsValid != tt.wantValid {
				t.Errorf("VerifyAddress() = %v, want %v", resp.IsValid, tt.wantValid)
			}
		})
	}
}

func TestVerifyContractAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		traceID   string
		wantValid bool
		wantErr   bool
	}{
		{
			name:      "valid contract address (UNI)",
			address:   "0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984",
			traceID:   "test-trace-id",
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "invalid address",
			address:   "0x1234",
			traceID:   "test-trace-id",
			wantValid: false,
			wantErr:   false,
		},
		{
			name:      "missing trace_id",
			address:   "0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984",
			traceID:   "",
			wantValid: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			config.SetConfigForTest(cfg)

			svc := &TxBuilderService{}

			req := &grpc.VerifyContractAddressRequest{
				TraceId: tt.traceID,
				Address: tt.address,
			}

			resp, err := svc.VerifyContractAddress(context.Background(), req)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyContractAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && resp.IsValid != tt.wantValid {
				t.Errorf("VerifyContractAddress() = %v, want %v", resp.IsValid, tt.wantValid)
			}
		})
	}
}

func TestCheckSufficientBalance_Validation(t *testing.T) {
	cfg := &config.Config{}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{}

	tests := []struct {
		name    string
		req     *grpc.CheckSufficientBalanceRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "",
				ChainCode:   "ethereum",
				Coin:        "eth",
				FromAddress: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				Amount:      "1000000000000000",
			},
			wantErr: true,
		},
		{
			name: "missing chain_code",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "",
				Coin:        "eth",
				FromAddress: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				Amount:      "1000000000000000",
			},
			wantErr: true,
		},
		{
			name: "missing coin",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "ethereum",
				Coin:        "",
				FromAddress: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				Amount:      "1000000000000000",
			},
			wantErr: true,
		},
		{
			name: "missing from_address",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "ethereum",
				Coin:        "eth",
				FromAddress: "",
				Amount:      "1000000000000000",
			},
			wantErr: true,
		},
		{
			name: "missing amount",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "ethereum",
				Coin:        "eth",
				FromAddress: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				Amount:      "",
			},
			wantErr: true,
		},
		// 注意：当 contract 为空时，代码不会返回错误，而是只检查主链币余额
		// 因此不需要 "token without contract" 的错误测试用例
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CheckSufficientBalance(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckSufficientBalance() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildSignRawData_Validation(t *testing.T) {
	cfg := &config.Config{}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{}

	tests := []struct {
		name    string
		req     *grpc.BuildSignRawDataRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &grpc.BuildSignRawDataRequest{
				TraceId:     "",
				ChainCode:   "ethereum",
				Coin:        "eth",
				CoinSymbol:  "ETH",
				FromAddress: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				ToAddress:   "0xAb5801a7D398351b8bE11C439e05C5B3259aeC9b",
				Amount:      "1000000000000000",
			},
			wantErr: true,
		},
		{
			name: "invalid from_address",
			req: &grpc.BuildSignRawDataRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "ethereum",
				Coin:        "eth",
				CoinSymbol:  "ETH",
				FromAddress: "invalid_address",
				ToAddress:   "0xAb5801a7D398351b8bE11C439e05C5B3259aeC9b",
				Amount:      "1000000000000000",
			},
			wantErr: true,
		},
		{
			name: "invalid to_address",
			req: &grpc.BuildSignRawDataRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "ethereum",
				Coin:        "eth",
				CoinSymbol:  "ETH",
				FromAddress: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				ToAddress:   "invalid_address",
				Amount:      "1000000000000000",
			},
			wantErr: true,
		},
		{
			name: "amount with decimal point",
			req: &grpc.BuildSignRawDataRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "ethereum",
				Coin:        "eth",
				CoinSymbol:  "ETH",
				FromAddress: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				ToAddress:   "0xAb5801a7D398351b8bE11C439e05C5B3259aeC9b",
				Amount:      "100.50",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.BuildSignRawData(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildSignRawData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTxBroadcast_Validation(t *testing.T) {
	cfg := &config.Config{}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{}

	tests := []struct {
		name    string
		req     *grpc.TxBroadcastRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &grpc.TxBroadcastRequest{
				TraceId:   "",
				RawData:   "0xf86c018504a817c80082520894d8da6bf26964af9d7eed9e03e53415d37aa960458088016345785d8a0000801ca0798c92bfb0d1dfccba6f0912a8f03d9e1bdfef5ee3de0bd67c7c5b97425d38b6a05a0c8b63e2c3d5e3f4f3e4f5f6f7f8f9fafbfcfdfeff0f1f2f3f4f5f6f7f8f9fafbfc",
				Signature: "abcd1234",
			},
			wantErr: true,
		},
		{
			name: "missing raw_data",
			req: &grpc.TxBroadcastRequest{
				TraceId:   "test-trace-id",
				RawData:   "",
				Signature: "abcd1234",
			},
			wantErr: true,
		},
		{
			name: "raw_data too long",
			req: &grpc.TxBroadcastRequest{
				TraceId:   "test-trace-id",
				RawData:   string(make([]byte, 4097)),
				Signature: "abcd1234",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.TxBroadcast(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("TxBroadcast() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPadAddressTo32Bytes(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantLen int // 期望返回的 hex 字符串长度（64 = 32 字节）
		wantErr bool
	}{
		{
			name:    "standard 40-char address",
			address: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
			wantLen: 64,
			wantErr: false,
		},
		{
			name:    "lowercase address",
			address: "0xd8da6bf26964af9d7eed9e03e53415d37aa96045",
			wantLen: 64,
			wantErr: false,
		},
		{
			name:    "without 0x prefix",
			address: "d8da6bf26964af9d7eed9e03e53415d37aa96045",
			wantLen: 64,
			wantErr: false,
		},
		{
			name:    "case insensitive",
			address: "0xD8DA6BF26964AF9D7EED9E03E53415D37AA96045",
			wantLen: 64,
			wantErr: false,
		},
		{
			name:    "invalid hex chars",
			address: "0xd8da6bf26964af9d7eed9e03e53415d37aaggggg",
			wantLen: 64,
			wantErr: true, // hex.DecodeString 会失败
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := padAddressTo32Bytes(tt.address)
			if len(result) != tt.wantLen {
				t.Errorf("padAddressTo32Bytes() len = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestPadAddressTo32BytesBytes(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantLen int // 期望返回的字节数组长度（32）
	}{
		{
			name:    "standard address",
			address: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
			wantLen: 32,
		},
		{
			name:    "lowercase",
			address: "0xd8da6bf26964af9d7eed9e03e53415d37aa96045",
			wantLen: 32,
		},
		{
			name:    "without 0x prefix",
			address: "d8da6bf26964af9d7eed9e03e53415d37aa96045",
			wantLen: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := padAddressTo32BytesBytes(tt.address)
			if len(result) != tt.wantLen {
				t.Errorf("padAddressTo32BytesBytes() len = %d, want %d", len(result), tt.wantLen)
			}

			// 验证前 12 字节是 0（地址是左补零的）
			for i := 0; i < 12; i++ {
				if result[i] != 0 {
					t.Errorf("padAddressTo32BytesBytes() padding[%d] = %x, want 0", i, result[i])
				}
			}

			// 验证最后 20 字节与非 hex 前缀版本匹配
			expectedAddr := strings.TrimPrefix(tt.address, "0x")
			expectedBytes, _ := hex.DecodeString(expectedAddr)
			for i, b := range expectedBytes {
				if result[12+i] != b {
					t.Errorf("padAddressTo32BytesBytes() addrBytes[%d] = %x, want %x", i, result[12+i], b)
				}
			}
		})
	}
}

func TestBuildERC20TransferData(t *testing.T) {
	to := "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
	amount := big.NewInt(7800000000000000000)

	data := buildERC20TransferData(to, amount)

	// 验证长度：4 字节 methodID + 32 字节地址 + 32 字节金额 = 68
	if len(data) != 68 {
		t.Errorf("buildERC20TransferData() len = %d, want 68", len(data))
	}

	// 验证 methodID
	expectedMethodID := []byte{0xa9, 0x05, 0x9c, 0xbb}
	for i, b := range expectedMethodID {
		if data[i] != b {
			t.Errorf("buildERC20TransferData() methodID[%d] = %x, want %x", i, data[i], b)
		}
	}

	// 验证前 12 字节是 0（地址是左补零的）
	for i := 0; i < 12; i++ {
		if data[4+i] != 0 {
			t.Errorf("buildERC20TransferData() padding[%d] = %x, want 0", i, data[4+i])
		}
	}
}
