package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/config"
	"github.com/koku-web3/txbuilder-ethereum/internal/ethereum"
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
			name:      "valid address lowercase",
			address:   "0xd8da6bf26964af9d7eed9e03e53415d37aa96045",
			traceID:   "test-trace-id",
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "valid address uppercase",
			address:   "0xD8DA6BF26964AF9D7EED9E03E53415D37AA96045",
			traceID:   "test-trace-id",
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "valid EIP-55 checksum address",
			address:   "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
			traceID:   "test-trace-id",
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "invalid EIP-55 checksum - modified last char",
			address:   "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96046",
			traceID:   "test-trace-id",
			wantValid: false,
			wantErr:   false,
		},
		{
			name:      "invalid EIP-55 checksum - wrong case at position 1",
			address:   "0xd8da6BF26964aF9D7eEd9e03E53415D37aA96045",
			traceID:   "test-trace-id",
			wantValid: false,
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

			req := &txbuilder.VerifyAddressRequest{
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

			req := &txbuilder.VerifyContractAddressRequest{
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
		req     *txbuilder.CheckSufficientBalanceRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &txbuilder.CheckSufficientBalanceRequest{
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
			req: &txbuilder.CheckSufficientBalanceRequest{
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
			req: &txbuilder.CheckSufficientBalanceRequest{
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
			req: &txbuilder.CheckSufficientBalanceRequest{
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
			req: &txbuilder.CheckSufficientBalanceRequest{
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
		req     *txbuilder.BuildSignRawDataRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &txbuilder.BuildSignRawDataRequest{
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
			req: &txbuilder.BuildSignRawDataRequest{
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
			req: &txbuilder.BuildSignRawDataRequest{
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
			req: &txbuilder.BuildSignRawDataRequest{
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
		req     *txbuilder.TxBroadcastRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &txbuilder.TxBroadcastRequest{
				TraceId:   "",
				RawData:   "0xf86c018504a817c80082520894d8da6bf26964af9d7eed9e03e53415d37aa960458088016345785d8a0000801ca0798c92bfb0d1dfccba6f0912a8f03d9e1bdfef5ee3de0bd67c7c5b97425d38b6a05a0c8b63e2c3d5e3f4f3e4f5f6f7f8f9fafbfcfdfeff0f1f2f3f4f5f6f7f8f9fafbfc",
				Signature: "abcd1234",
			},
			wantErr: true,
		},
		{
			name: "missing raw_data",
			req: &txbuilder.TxBroadcastRequest{
				TraceId:   "test-trace-id",
				RawData:   "",
				Signature: "abcd1234",
			},
			wantErr: true,
		},
		{
			name: "raw_data too long",
			req: &txbuilder.TxBroadcastRequest{
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

func TestConvertAddress_Validation(t *testing.T) {
	cfg := &config.Config{}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{}

	validPEM := generateTestSecp256k1PEM(t)

	tests := []struct {
		name    string
		req     *txbuilder.ConvertAddressRequest
		wantErr bool
	}{
		{
			name: "missing trace_id",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: "",
				Keys:    []*txbuilder.PublicKeysRequest{{AccountIndex: 0, PkixPubkeyPem: validPEM}},
			},
			wantErr: true,
		},
		{
			name: "trace_id too long (>36)",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: strings.Repeat("a", 37),
				Keys:    []*txbuilder.PublicKeysRequest{{AccountIndex: 0, PkixPubkeyPem: validPEM}},
			},
			wantErr: true,
		},
		{
			name: "empty keys",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: "trace-001",
				Keys:    []*txbuilder.PublicKeysRequest{},
			},
			wantErr: true,
		},
		{
			name: "keys length > 100",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: "trace-001",
				Keys:    generateKeys(101, validPEM),
			},
			wantErr: true,
		},
		{
			name: "empty pkix_pubkey_pem",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: "trace-001",
				Keys:    []*txbuilder.PublicKeysRequest{{AccountIndex: 0, PkixPubkeyPem: ""}},
			},
			wantErr: true,
		},
		{
			name: "whitespace-only pkix_pubkey_pem",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: "trace-001",
				Keys:    []*txbuilder.PublicKeysRequest{{AccountIndex: 0, PkixPubkeyPem: "   "}},
			},
			wantErr: true,
		},
		{
			name: "pkix_pubkey_pem too long (>4096)",
			req: &txbuilder.ConvertAddressRequest{
				TraceId: "trace-001",
				Keys:    []*txbuilder.PublicKeysRequest{{AccountIndex: 0, PkixPubkeyPem: strings.Repeat("a", 4097)}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ConvertAddress(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvertAddress_Success(t *testing.T) {
	cfg := &config.Config{}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{}

	// Generate a real secp256k1 key and derive the expected address.
	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("crypto.GenerateKey failed: %v", err)
	}
	pubKey := privKey.Public().(*ecdsa.PublicKey)
	expectedAddr := crypto.PubkeyToAddress(*pubKey)

	validPEM, err := ethereum.MarshalPublicKeyToPKIXPEM(pubKey)
	if err != nil {
		t.Fatalf("MarshalPublicKeyToPKIXPEM failed: %v", err)
	}

	req := &txbuilder.ConvertAddressRequest{
		TraceId: "trace-001",
		Keys: []*txbuilder.PublicKeysRequest{
			{AccountIndex: 0, PkixPubkeyPem: validPEM},
			{AccountIndex: 5, PkixPubkeyPem: validPEM},
		},
	}

	resp, err := svc.ConvertAddress(context.Background(), req)
	if err != nil {
		t.Fatalf("ConvertAddress() unexpected error: %v", err)
	}

	if len(resp.Keys) != 2 {
		t.Fatalf("ConvertAddress() returned %d keys, want 2", len(resp.Keys))
	}

	for _, key := range resp.Keys {
		if !ethereum.ValidateAddress(key.Address) {
			t.Errorf("ConvertAddress() returned invalid Ethereum address: %q", key.Address)
		}
		if key.AccountIndex != 0 && key.AccountIndex != 5 {
			t.Errorf("ConvertAddress() unexpected account_index: %d", key.AccountIndex)
		}
		// Each result should be the same derived address (same PEM input), with EIP-55 checksum.
		expectedChecksumAddr := ethereum.ConvertToChecksumAddress(expectedAddr.Hex())
		if key.Address != expectedChecksumAddr {
			t.Errorf("ConvertAddress() address = %q, want %q", key.Address, expectedChecksumAddr)
		}
	}
}

func TestConvertAddress_InvalidPEM(t *testing.T) {
	cfg := &config.Config{}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{}

	req := &txbuilder.ConvertAddressRequest{
		TraceId: "trace-001",
		Keys:    []*txbuilder.PublicKeysRequest{{AccountIndex: 0, PkixPubkeyPem: "not-a-valid-pem"}},
	}

	_, err := svc.ConvertAddress(context.Background(), req)
	if err == nil {
		t.Error("ConvertAddress() expected error for invalid PEM, got nil")
	}
}

func generateTestSecp256k1PEM(t *testing.T) string {
	t.Helper()
	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("crypto.GenerateKey failed: %v", err)
	}
	pubKey := privKey.Public().(*ecdsa.PublicKey)
	pemStr, err := ethereum.MarshalPublicKeyToPKIXPEM(pubKey)
	if err != nil {
		t.Fatalf("MarshalPublicKeyToPKIXPEM failed: %v", err)
	}
	return pemStr
}

func generateKeys(n int, pemStr string) []*txbuilder.PublicKeysRequest {
	keys := make([]*txbuilder.PublicKeysRequest, n)
	for i := 0; i < n; i++ {
		keys[i] = &txbuilder.PublicKeysRequest{AccountIndex: uint32(i), PkixPubkeyPem: pemStr}
	}
	return keys
}

func TestConvertAddress_NonSecp256k1Key(t *testing.T) {
	cfg := &config.Config{}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{}

	// Generate a P-256 (NIST P-256) key, not secp256k1.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	pemStr, err := ethereum.MarshalPublicKeyToPKIXPEM(&privKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPublicKeyToPKIXPEM failed: %v", err)
	}

	req := &txbuilder.ConvertAddressRequest{
		TraceId: "trace-001",
		Keys:    []*txbuilder.PublicKeysRequest{{AccountIndex: 0, PkixPubkeyPem: pemStr}},
	}

	_, err = svc.ConvertAddress(context.Background(), req)
	if err == nil {
		t.Error("ConvertAddress() expected error for non-secp256k1 key, got nil")
	}
}
