package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TxBroadcast 广播已签名的交易到以太坊网络
// 将签名后的交易提交到节点，节点验证后放入交易池并执行
func (s *TxBuilderService) TxBroadcast(ctx context.Context, req *txbuilder.TxBroadcastRequest) (*txbuilder.TxBroadcastResponse, error) {
	log.Info("[TxBroadcast] Request received", "trace_id", req.TraceId, "raw_data_length", len(req.RawData), "signature_length", len(req.Signature))

	if req.TraceId == "" {
		log.Warn("[TxBroadcast] Missing trace_id", "trace_id", req.TraceId)
		return nil, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if req.RawData == "" {
		log.Warn("[TxBroadcast] Missing raw_data", "trace_id", req.TraceId)
		return nil, status.Error(codes.InvalidArgument, "raw_data is required")
	}

	if len(req.RawData) > 4096 {
		log.Warn("[TxBroadcast] Invalid raw_data length", "trace_id", req.TraceId, "raw_data_length", len(req.RawData))
		return nil, status.Error(codes.InvalidArgument, "raw_data must be 1-4096 characters")
	}

	log.Info("[TxBroadcast] Validation passed, broadcasting transaction", "trace_id", req.TraceId)

	signatureBytes, err := hex.DecodeString(req.Signature)
	if err != nil {
		log.Warn("[TxBroadcast] Invalid request payload", "trace_id", req.TraceId, "stage", "decode_signature", "error", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("hex decode from Signature failed: %s", err.Error()))
	}
	if len(signatureBytes) != 65 {
		log.Warn("[TxBroadcast] Invalid request payload", "trace_id", req.TraceId, "stage", "validate_signature_length", "actual_length", len(signatureBytes), "expected_length", 65)
		return nil, status.Error(codes.InvalidArgument, "signature's length must be 65 bytes")
	}

	txBytes, err := hex.DecodeString(req.RawData)
	if err != nil {
		log.Warn("[TxBroadcast] Invalid request payload", "trace_id", req.TraceId, "stage", "decode_raw_data", "error", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("hex decode from raw data failed: %s", err.Error()))
	}

	tx := &types.Transaction{}
	if err := tx.UnmarshalBinary(txBytes); err != nil {
		log.Warn("[TxBroadcast] Invalid request payload", "trace_id", req.TraceId, "stage", "unmarshal_transaction", "error", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("raw data unmarshalBinary to transaction failed: %s", err.Error()))
	}

	signer := types.NewLondonSigner(big.NewInt(int64(s.rpc.ChainID)))

	signedTx, err := tx.WithSignature(signer, signatureBytes)
	if err != nil {
		log.Warn("[TxBroadcast] Invalid request payload", "trace_id", req.TraceId, "stage", "apply_signature", "error", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("tx withSignature failed: %s", err.Error()))
	}

	signedTxBytes, err := signedTx.MarshalBinary()
	if err != nil {
		log.Warn("[TxBroadcast] Invalid request payload", "trace_id", req.TraceId, "stage", "marshal_signed_transaction", "error", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("signedTx MarshalBinary failed: %s", err.Error()))
	}

	txHash, err := s.rpc.BroadcastRawTransaction(ctx, hex.EncodeToString(signedTxBytes))
	if err != nil {
		log.Error("[TxBroadcast] Failed to broadcast transaction", "trace_id", req.TraceId, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to broadcast transaction: %v", err)
	}

	log.Info("[TxBroadcast] Broadcast succeeded", "trace_id", req.TraceId, "tx_hash", txHash)

	return &txbuilder.TxBroadcastResponse{Success: true, TxHash: txHash}, nil
}
