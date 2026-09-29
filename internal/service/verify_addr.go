package service

import (
	"context"

	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/ethereum"
	"github.com/koku-web3/txbuilder-ethereum/internal/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// VerifyAddress 验证普通以太坊地址格式（EOA 地址）
// 支持 EIP-55 校验和格式验证：
// - 如果地址是 EIP-55 格式（混合大小写），则验证校验和是否正确
// - 如果地址是纯小写或纯大写，则只验证基本格式
func (s *TxBuilderService) VerifyAddress(ctx context.Context, req *txbuilder.VerifyAddressRequest) (*txbuilder.VerifyAddressResponse, error) {
	log.Debug("VerifyAddress received", "params", req)
	isValid, err := s.verifyAddress("VerifyAddress", req.TraceId, req.Address, ethereum.ValidateAddress)
	if err != nil {
		log.Warn("Input validation failed", "trace_id", req.TraceId, "address", req.Address)
		return nil, errors.InvalidArgument(err.Error())
	}
	if !isValid {
		return &txbuilder.VerifyAddressResponse{IsValid: false}, nil
	}

	// 如果地址是 EIP-55 格式，验证校验和
	if ethereum.IsEIP55Format(req.Address) {
		isValid = ethereum.VerifyChecksum(req.Address)
	}
	log.Info("VerifyContractAddress success!", "trace_id", req.TraceId, "address", req.Address, "is_valid", isValid)
	return &txbuilder.VerifyAddressResponse{IsValid: isValid}, nil
}

// verifyAddress 验证地址的通用逻辑
// validateFn 是具体的验证函数（如 ValidateAddress 或 ValidateContractAddress）
func (s *TxBuilderService) verifyAddress(methodName, traceId, address string, validateFn func(string) bool) (bool, error) {
	if traceId == "" {
		return false, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if address == "" {
		log.Warn("["+methodName+"] Missing address", "trace_id", traceId, "address", address)
		return false, status.Error(codes.InvalidArgument, "address is required")
	}

	isValid := validateFn(address)
	return isValid, nil
}
