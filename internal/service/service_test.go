package service

import (
	"context"
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
				CoinId:      "eth",
				IsBasicCoin: true,
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
				CoinId:      "eth",
				IsBasicCoin: true,
				FromAddress: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				Amount:      "1000000000000000",
			},
			wantErr: true,
		},
		{
			name: "missing coin_id",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "ethereum",
				CoinId:      "",
				IsBasicCoin: true,
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
				CoinId:      "eth",
				IsBasicCoin: true,
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
				CoinId:      "eth",
				IsBasicCoin: true,
				FromAddress: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				Amount:      "",
			},
			wantErr: true,
		},
		{
			name: "token without contract",
			req: &grpc.CheckSufficientBalanceRequest{
				TraceId:     "test-trace-id",
				ChainCode:   "ethereum",
				CoinId:      "usdt",
				IsBasicCoin: false,
				FromAddress: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				Amount:      "1000000",
				Contract:    "",
			},
			wantErr: true,
		},
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
			name: "missing biz_id",
			req: &grpc.BuildSignRawDataRequest{
				BizId:       "",
				ChainCode:   "ethereum",
				CoinId:      "eth",
				IsBasicCoin: true,
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
				BizId:       "test-biz-id",
				ChainCode:   "ethereum",
				CoinId:      "eth",
				IsBasicCoin: true,
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
				BizId:       "test-biz-id",
				ChainCode:   "ethereum",
				CoinId:      "eth",
				IsBasicCoin: true,
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
				BizId:       "test-biz-id",
				ChainCode:   "ethereum",
				CoinId:      "eth",
				IsBasicCoin: true,
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
