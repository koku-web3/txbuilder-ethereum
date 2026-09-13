package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/koku-web3/logko"
	"github.com/koku-web3/txbuilder-ethereum/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TxBroadcast 广播已签名的交易到以太坊网络
// 将签名后的交易提交到节点，节点验证后放入交易池并执行
func (s *TxBuilderService) TxBroadcast(ctx context.Context, req *grpc.TxBroadcastRequest) (*grpc.TxBroadcastResponse, error) {
	log.Info("[TxBroadcast] received request", "trace_id", req.TraceId, "raw_data_length", len(req.RawData), "signature_length", len(req.Signature))

	if req.TraceId == "" {
		log.Warn("[TxBroadcast] missing trace_id", "trace_id", req.TraceId)
		return nil, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if req.RawData == "" {
		log.Warn("[TxBroadcast] missing raw_data", "trace_id", req.TraceId)
		return nil, status.Error(codes.InvalidArgument, "raw_data is required")
	}

	if len(req.RawData) > 4096 {
		log.Warn("[TxBroadcast] invalid raw_data length", "trace_id", req.TraceId, "raw_data_length", len(req.RawData))
		return nil, status.Error(codes.InvalidArgument, "raw_data must be 1-4096 characters")
	}

	log.Info("[TxBroadcast] validation passed, broadcasting transaction", "trace_id", req.TraceId)

	signatureBytes, err := hex.DecodeString(req.Signature)
	if err != nil {
		log.Warn("[TxBroadcast] hex decode from Signature failed", "trace_id", req.TraceId, "err", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("hex decode from Signature failed: %s", err.Error()))
	}
	if len(signatureBytes) != 65 {
		log.Warn("[TxBroadcast] invalid signature length", "trace_id", req.TraceId, "signature", len(signatureBytes))
		return nil, status.Error(codes.InvalidArgument, "signature's length must be 65 bytes")
	}

	txBytes, err := hex.DecodeString(req.RawData)
	if err != nil {
		log.Warn("[TxBroadcast] hex decode from raw data failed", "trace_id", req.TraceId, "err", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("hex decode from raw data failed: %s", err.Error()))
	}

	tx := &types.Transaction{}
	if err := tx.UnmarshalBinary(txBytes); err != nil {
		log.Warn("[TxBroadcast] raw data unmarshalBinary to transaction failed", "trace_id", req.TraceId, "err", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("raw data unmarshalBinary to transaction failed: %s", err.Error()))
	}

	signer := types.NewLondonSigner(big.NewInt(int64(s.rpc.ChainID)))

	signedTx, err := tx.WithSignature(signer, signatureBytes)
	if err != nil {
		log.Warn("[TxBroadcast] tx withSignature failed", "trace_id", req.TraceId, "err", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("tx withSignature failed: %s", err.Error()))
	}

	signedTxBytes, err := signedTx.MarshalBinary()
	if err != nil {
		log.Warn("[TxBroadcast] signedTx MarshalBinary failed", "trace_id", req.TraceId, "err", err.Error())
		return nil, status.Error(codes.Internal, fmt.Sprintf("signedTx MarshalBinary failed: %s", err.Error()))
	}

	txHash, err := s.rpc.BroadcastRawTransaction(ctx, hex.EncodeToString(signedTxBytes))
	if err != nil {
		log.Error("[TxBroadcast] failed to broadcast transaction", "trace_id", req.TraceId, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to broadcast transaction: %v", err)
	}

	log.Info("[TxBroadcast] broadcast success", "trace_id", req.TraceId, "tx_hash", txHash, "success", true)

	return &grpc.TxBroadcastResponse{Success: true, TxHash: txHash}, nil
}
