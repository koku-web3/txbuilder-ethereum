package service

import (
	"context"
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/pkg/errors"
)

// TxBroadcast 广播已签名的交易到以太坊网络
// 将签名后的交易提交到节点，节点验证后放入交易池并执行
func (s *TxBuilderService) TxBroadcast(ctx context.Context, req *txbuilder.TxBroadcastRequest) (*txbuilder.TxBroadcastResponse, error) {
	log.Debug("TxBroadcast received", "params", req)

	if err := s.validateTxBroadcast(req); err != nil {
		log.Warn("Input validation failed", "trace_id", req.TraceId, "error", err.Error())
		return nil, errors.InvalidArgument(err.Error())
	}

	signatureBytes, err := hex.DecodeString(req.Signature)
	if err != nil {
		return nil, errors.InvalidArgumentf("hex decode from Signature failed: %v", err)
	}
	if len(signatureBytes) != 65 {
		return nil, errors.InvalidArgument("signature's length must be 65 bytes")
	}

	txBytes, err := hex.DecodeString(req.RawData)
	if err != nil {
		return nil, errors.InvalidArgumentf("hex decode from raw data failed: %v", err)
	}

	tx := &types.Transaction{}
	if err := tx.UnmarshalBinary(txBytes); err != nil {
		log.Warn("Invalid request payload, failed to unmarshalBinary rawData", "trace_id", req.TraceId, "error", err.Error())
		return nil, errors.InvalidArgumentf("raw data unmarshalBinary to transaction failed: %v", err)
	}

	signer := types.NewLondonSigner(big.NewInt(int64(s.rpc.ChainID)))

	signedTx, err := tx.WithSignature(signer, signatureBytes)
	if err != nil {
		log.Warn("Invalid request payload, failed to combine signer and signature", "trace_id", req.TraceId, "error", err.Error())
		return nil, errors.Internalf("tx withSignature failed: %v", err)
	}

	signedTxBytes, err := signedTx.MarshalBinary()
	if err != nil {
		log.Error("MarshalBinary signed tx failed", "trace_id", req.TraceId, "error", err.Error())
		return nil, errors.Internalf("signedTx MarshalBinary failed: %v", err)
	}

	txHash, err := s.rpc.BroadcastRawTransaction(ctx, hex.EncodeToString(signedTxBytes))
	if err != nil {
		log.Error("Broadcast transaction failed", "trace_id", req.TraceId, "error", err)
		return nil, errors.Internalf("failed to broadcast transaction: %v", err)
	}

	log.Info("Broadcast tx succeeded", "trace_id", req.TraceId, "tx_hash", txHash)

	return &txbuilder.TxBroadcastResponse{TxHash: txHash}, nil
}

func (s *TxBuilderService) validateTxBroadcast(req *txbuilder.TxBroadcastRequest) error {
	if req.TraceId == "" {
		return errors.New("trace_id is required and must be 1-36 characters")
	}
	if req.RawData == "" {
		return errors.New("raw_data is required")
	}

	if len(req.RawData) > 4096 {
		return errors.New("raw_data must be 1-4096 characters")
	}
	return nil
}
