package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/pkg/errors"
)

// TxBroadcast 广播已签名的交易到以太坊网络
// 将签名后的交易提交到节点，节点验证后放入交易池并执行
func (s *TxBuilderService) TxBroadcast(ctx context.Context, req *txbuilder.TxBroadcastRequest) (*txbuilder.TxBroadcastResponse, error) {
	log.Debug("TxBroadcast received", "trace_id", req.TraceId, "signature", req.Signature, "raw_data", req.RawData)

	if err := validateTxBroadcast(req.TraceId, req.RawData); err != nil {
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
		return nil, errors.Internal()
	}

	signedTxBytes, err := signedTx.MarshalBinary()
	if err != nil {
		log.Error("MarshalBinary signed tx failed", "trace_id", req.TraceId, "chain_id", chainID, "signature", req.Signature, "error", err)
		return nil, errors.Internal()
	}

	txHash, err := s.rpc.BroadcastRawTransaction(ctx, hex.EncodeToString(signedTxBytes))
	if err != nil {
		log.Error("Broadcast transaction failed", "trace_id", req.TraceId, "error", err)
		return nil, errors.Internal()
	}

	log.Info("Broadcast tx succeeded", "trace_id", req.TraceId, "tx_hash", txHash)

	return &txbuilder.TxBroadcastResponse{TxHash: txHash}, nil
}

func validateTxBroadcast(traceID, rawData string) error {
	if traceID == "" {
		return errors.New("trace_id is required and must be 1-36 characters")
	}
	if rawData == "" {
		return errors.New("raw_data is required")
	}

	if len(rawData) > 4096 {
		return errors.New("raw_data must be 1-4096 characters")
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
