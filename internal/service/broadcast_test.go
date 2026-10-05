package service

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/config"
	"github.com/koku-web3/txbuilder-ethereum/internal/ethereum"
)

// testChainID 与 config/config.toml 中的 chain_id 保持一致（Sepolia）
const testChainID = uint64(11155111)

// buildTestDynamicFeeTx 构造一笔未签名的 EIP-1559 交易，结构与
// buildTokenTransaction / buildBasicCoinTransaction 保持一致。
func buildTestDynamicFeeTx(t *testing.T, chainID *big.Int) *types.Transaction {
	t.Helper()

	toAddr := common.HexToAddress("0xab5801a7d398351b8be11c439e05c5b3259aec9b")
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     0,
		GasTipCap: big.NewInt(1_000_000_000),
		GasFeeCap: big.NewInt(2_000_000_000),
		Gas:       56800,
		To:        &toAddr,
		Value:     big.NewInt(0),
	})
}

// signTestTx 用私钥对交易签名，返回 (r||s||v) 65 字节 hex 字符串。
// v 保持 0/1 的 recovery id 形式，与 proto 约定一致。
func signTestTx(t *testing.T, tx *types.Transaction, privKey *ecdsa.PrivateKey) string {
	t.Helper()

	signer := types.NewLondonSigner(tx.ChainId())
	sig, err := crypto.Sign(signer.Hash(tx).Bytes(), privKey)
	if err != nil {
		t.Fatalf("crypto.Sign failed: %v", err)
	}
	// crypto.Sign 返回的最后一字节是 0/1，无需转换，可直接 hex 编码
	return hex.EncodeToString(sig)
}

func TestValidateTxBroadcast_FromAddress(t *testing.T) {
	const validAddr = "0xd8da6bf26964af9d7eed9e03e53415d37aa96045"

	tests := []struct {
		name        string
		fromAddress string
		wantErr     bool
	}{
		{
			name:        "valid lowercase address",
			fromAddress: validAddr,
			wantErr:     false,
		},
		{
			name:        "valid EIP-55 checksum address",
			fromAddress: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
			wantErr:     false,
		},
		{
			name:        "missing from_address",
			fromAddress: "",
			wantErr:     true,
		},
		{
			name:        "malformed address (too short)",
			fromAddress: "0x1234",
			wantErr:     true,
		},
		{
			name:        "malformed address (no 0x prefix)",
			fromAddress: "d8da6bf26964af9d7eed9e03e53415d37aa96045",
			wantErr:     true,
		},
		{
			name:        "malformed address (non-hex chars)",
			fromAddress: "0xzzda6bf26964af9d7eed9e03e53415d37aa96045",
			wantErr:     true,
		},
		{
			name:        "from_address exceeds 256 chars",
			fromAddress: "0x" + strings.Repeat("a", 300),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTxBroadcast("trace-001", "0xdeadbeef", tt.fromAddress)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTxBroadcast() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestTxBroadcast_SenderRecoveryRoundTrip 确认校验对「正确签名」是放行的，
// 防止把检查写成永远拒绝。同时覆盖 from_address 的大小写兼容性：
// sender.Hex() 返回 EIP-55 混合大小写，调用方传全小写也应通过。
func TestTxBroadcast_SenderRecoveryRoundTrip(t *testing.T) {
	cfg := &config.Config{Chain: config.ChainConfig{ChainID: testChainID}}
	config.SetConfigForTest(cfg)

	chainID := big.NewInt(int64(testChainID))
	tx := buildTestDynamicFeeTx(t, chainID)

	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("crypto.GenerateKey failed: %v", err)
	}
	expectedAddr := crypto.PubkeyToAddress(privKey.PublicKey)

	signature := signTestTx(t, tx, privKey)
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		t.Fatalf("hex.DecodeString(signature) failed: %v", err)
	}

	signer := types.NewLondonSigner(chainID)
	signedTx, err := tx.WithSignature(signer, sigBytes)
	if err != nil {
		t.Fatalf("WithSignature failed: %v", err)
	}

	sender, err := types.Sender(signer, signedTx)
	if err != nil {
		t.Fatalf("types.Sender failed: %v", err)
	}
	if sender != expectedAddr {
		t.Fatalf("Sender() = %s, want %s", sender.Hex(), expectedAddr.Hex())
	}

	// 与 broadcast.go 中的比对逻辑保持一致：EIP-55 校验和格式与全小写都应放行
	for _, fromAddr := range []string{
		expectedAddr.Hex(),
		strings.ToLower(expectedAddr.Hex()),
	} {
		if !strings.EqualFold(sender.Hex(), fromAddr) {
			t.Errorf("address comparison failed for %s vs recovered %s", fromAddr, sender.Hex())
		}
	}
}

// TestTxBroadcast_RejectsSignatureSenderMismatch 覆盖本次加固的核心场景：
// 签名本身格式合法，但签署的摘要与交易不匹配，导致 ecrecover 恢复出
// 另一个地址。若无此校验，这类请求会一路飘到节点，并被报成
// "insufficient funds for gas * price + value: balance 0"。
func TestTxBroadcast_RejectsSignatureSenderMismatch(t *testing.T) {
	cfg := &config.Config{Chain: config.ChainConfig{ChainID: testChainID}}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{rpc: ethereum.NewRPCClient()}

	chainID := big.NewInt(int64(testChainID))
	tx := buildTestDynamicFeeTx(t, chainID)
	rawData, err := tx.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}

	// 用「另一个」私钥对同一笔交易签名：签名合法，但恢复出的地址不是 fromAddress
	otherKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("crypto.GenerateKey failed: %v", err)
	}
	signature := signTestTx(t, tx, otherKey)

	intendedAddr := common.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")

	// 广播调用会在本地被拒绝，不会真正打到节点
	_, err = svc.TxBroadcast(context.Background(), &txbuilder.TxBroadcastRequest{
		TraceId:     "trace-mismatch",
		RawData:     hex.EncodeToString(rawData),
		Signature:   signature,
		FromAddress: intendedAddr.Hex(),
	})

	if err == nil {
		t.Fatal("TxBroadcast() expected error for mismatched signature sender, got nil")
	}
	if !strings.Contains(err.Error(), "signature does not match from_address") {
		t.Errorf("TxBroadcast() error = %v, want a signature/from_address mismatch error", err)
	}
	// 错误信息必须带出实际恢复到的地址，便于直接定位问题
	recovered, rErr := types.Sender(types.NewLondonSigner(chainID), func() *types.Transaction {
		txb, _ := tx.WithSignature(types.NewLondonSigner(chainID), mustDecodeHex(t, signature))
		return txb
	}())
	if rErr == nil && !strings.Contains(err.Error(), recovered.Hex()) {
		t.Errorf("error message does not echo recovered address %s: %v", recovered.Hex(), err)
	}
}

// mustDecodeHex 是测试辅助函数，hex 解码失败直接终止测试。
func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex.DecodeString(%q) failed: %v", s, err)
	}
	return b
}

// TestTxBroadcast_RejectsMalformedSignature 确认无法恢复出合法 sender 的签名
// 被归类为 InvalidArgument，而不是被当作内部错误。
func TestTxBroadcast_RejectsMalformedSignature(t *testing.T) {
	cfg := &config.Config{Chain: config.ChainConfig{ChainID: testChainID}}
	config.SetConfigForTest(cfg)

	svc := &TxBuilderService{rpc: ethereum.NewRPCClient()}

	chainID := big.NewInt(int64(testChainID))
	tx := buildTestDynamicFeeTx(t, chainID)
	rawData, err := tx.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}

	// 全零签名：长度合法但 r=0，ecrecover 会失败
	zeroSig := strings.Repeat("00", 65)

	_, err = svc.TxBroadcast(context.Background(), &txbuilder.TxBroadcastRequest{
		TraceId:     "trace-badsig",
		RawData:     hex.EncodeToString(rawData),
		Signature:   zeroSig,
		FromAddress: common.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045").Hex(),
	})

	if err == nil {
		t.Fatal("TxBroadcast() expected error for unrecoverable signature, got nil")
	}
}
