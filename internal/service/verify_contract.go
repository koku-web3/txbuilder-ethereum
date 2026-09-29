package service

import (
	"context"

	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/ethereum"
	"github.com/koku-web3/txbuilder-ethereum/internal/pkg/errors"
)

// VerifyContractAddress 验证智能合约地址格式
// 智能合约地址与普通地址格式相同（0x 开头 + 40 位十六进制），但需要通过 eth_call 验证合约是否存在
func (s *TxBuilderService) VerifyContractAddress(ctx context.Context, req *txbuilder.VerifyContractAddressRequest) (*txbuilder.VerifyContractAddressResponse, error) {
	log.Debug("VerifyContractAddress received", "params", req)
	isValid, err := s.verifyAddress("VerifyContractAddress", req.TraceId, req.Address, ethereum.ValidateContractAddress)
	if err != nil {
		log.Warn("Input validation failed", "trace_id", req.TraceId, "address", req.Address)
		return nil, errors.InvalidArgument(err.Error())
	}
	log.Info("VerifyContractAddress success!", "trace_id", req.TraceId, "contract", req.Address, "is_valid", isValid)
	return &txbuilder.VerifyContractAddressResponse{IsValid: isValid}, nil
}
