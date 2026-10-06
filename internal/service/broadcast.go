package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/ethereum"
	"github.com/koku-web3/txbuilder-ethereum/internal/pkg/errors"
)

// TxBroadcast 广播已签名的交易到以太坊网络
// 将签名后的交易提交到节点，节点验证后放入交易池并执行
func (s *TxBuilderService) TxBroadcast(ctx context.Context, req *txbuilder.TxBroadcastRequest) (*txbuilder.TxBroadcastResponse, error) {
	log.Debug("TxBroadcast received", "trace_id", req.TraceId, "from_address", req.FromAddress, "signature", req.Signature, "raw_data", req.RawData)

	if err := validateTxBroadcast(req.TraceId, req.RawData, req.FromAddress); err != nil {
		log.Warn("Invalid input parameter", "trace_id", req.TraceId, "params", req.RawData, "error", err)
		return nil, errors.InvalidArgumentErr(err)
	}

	signatureBytes, err := decodeSignature(req.Signature)
	if err != nil {
		log.Warn("Decode input parameter from hex to string failed", "trace_id", req.TraceId, "params", req.Signature, "error", err)
		return nil, errors.InvalidArgument("signature")
	}

	txBytes, err := hex.DecodeString(req.RawData)
	if err != nil {
		log.Warn("Decode input parameter from hex to string failed", "trace_id", req.TraceId, "params", req.RawData, "error", err)
		return nil, errors.InvalidArgument("raw_data")
	}

	tx := &types.Transaction{}
	if err := tx.UnmarshalBinary(txBytes); err != nil {
		log.Warn("Invalid request payload, failed to unmarshalBinary", "trace_id", req.TraceId, "params", req.RawData, "error", err)
		return nil, errors.InvalidArgument("raw_data")
	}

	chainID := int64(s.rpc.ChainID)
	signer := types.NewLondonSigner(big.NewInt(chainID))

	signedTx, err := tx.WithSignature(signer, signatureBytes)
	if err != nil {
		log.Error("Invalid request payload, failed to combine signer and signature", "trace_id", req.TraceId, "error", err)
		return nil, errors.Internal("failed to sign tx.")
	}

	// 关键安全校验：反推交易发起方并与调用方声明的 from_address 比对。
	//
	// WithSignature 只做 R/S/V 的格式组装，不会验证签名与交易哈希是否匹配。
	// 若外部（Coordinator/KMS）签名的是错误的摘要（例如把 raw_data 而非 msg 送进签名，
	// 或对字符串而非 32 字节做了额外哈希），ecrecover 依然会成功，但恢复出的是另一个
	// 与预期无关的地址。此时若直接广播，节点会以
	// "insufficient funds for gas * price + value: balance 0" 这种极具误导性的错误拒绝，
	// 因为那个被误恢复出来的地址往往余额为 0。
	// 在本地拦下它，才能给出直指要害的报错。
	sender, err := types.Sender(signer, signedTx)
	if err != nil {
		log.Warn("Failed to recover transaction sender from signature", "trace_id", req.TraceId, "error", err)
		return nil, errors.InvalidArgument("signature")
	}

	if !strings.EqualFold(sender.Hex(), req.FromAddress) {
		log.Warn("Signature sender mismatch, request rejected",
			"trace_id", req.TraceId,
			"expected_from_address", req.FromAddress,
			"recovered_sender", sender.Hex())
		return nil, errors.InvalidArgumentf(
			"signature does not match from_address: signature recovers to %s, but from_address is %s; "+
				"the signature was likely produced over the wrong digest (expected the 32-byte msg from BuildSignRawData)",
			sender.Hex(), req.FromAddress)
	}

	signedTxBytes, err := signedTx.MarshalBinary()
	if err != nil {
		log.Error("MarshalBinary signed tx failed", "trace_id", req.TraceId, "chain_id", chainID, "signature", req.Signature, "error", err)
		return nil, errors.Internal("failed to marshalBinary signed tx.")
	}

	txHash, err := s.rpc.BroadcastRawTransaction(ctx, hex.EncodeToString(signedTxBytes))
	if err != nil {
		log.Error("Broadcast transaction failed", "trace_id", req.TraceId, "error", err)
		return nil, errors.Internal("failed to broadcast tx.")
	}

	log.Info("Broadcast tx succeeded", "trace_id", req.TraceId, "tx_hash", txHash)

	return &txbuilder.TxBroadcastResponse{TxHash: txHash}, nil
}

func validateTxBroadcast(traceID, rawData, fromAddress string) error {
	if traceID == "" {
		return errors.New("trace_id is required and must be 1-36 characters")
	}
	if rawData == "" {
		return errors.New("raw_data is required")
	}

	if len(rawData) > 4096 {
		return errors.New("raw_data must be 1-4096 characters")
	}
	if fromAddress == "" {
		return errors.New("from_address is required")
	}
	if len(fromAddress) > 256 {
		return errors.New("from_address must be 1-256 characters")
	}
	// 提前拦截格式非法的地址，避免后面拿它跟 ecrecover 的结果做无意义的比对
	if !ethereum.ValidateAddress(fromAddress) {
		return errors.New("invalid from_address")
	}
	return nil
}

func decodeSignature(s string) ([]byte, error) {
	signatureBytes, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("hex decode from Signature failed: %v", err)
	}
	if len(signatureBytes) != 65 {
		return nil, fmt.Errorf("signature's length must be %d bytes", 65)
	}
	return signatureBytes, nil
}
