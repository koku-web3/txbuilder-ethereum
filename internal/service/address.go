package service

import (
	"context"
	"strings"

	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/ethereum"
	"github.com/koku-web3/txbuilder-ethereum/internal/pkg/errors"
)

// VerifyAddress 验证普通以太坊地址格式（EOA 地址）
// 支持 EIP-55 校验和格式验证：
// - 如果地址是 EIP-55 格式（混合大小写），则验证校验和是否正确
// - 如果地址是纯小写或纯大写，则只验证基本格式
func (s *TxBuilderService) VerifyAddress(ctx context.Context, req *txbuilder.VerifyAddressRequest) (*txbuilder.VerifyAddressResponse, error) {
	log.Debug("VerifyAddress received", "trace_id", req.TraceId, "address", req.Address)

	if err := verifyAddressBasic(req.TraceId, req.Address); err != nil {
		log.Warn("Invalid input parameter", "trace_id", req.TraceId, "error", err)
		return nil, errors.InvalidArgumentErr(err)
	}

	isValid := ethereum.ValidateAddress(req.Address)
	if !isValid {
		return &txbuilder.VerifyAddressResponse{IsValid: false}, nil
	}

	// 如果地址是 EIP-55 格式，验证校验和
	if ethereum.IsEIP55Format(req.Address) {
		isValid = ethereum.VerifyChecksum(req.Address)
	}
	log.Info("VerifyAddress success!", "trace_id", req.TraceId, "address", req.Address, "is_valid", isValid)
	return &txbuilder.VerifyAddressResponse{IsValid: isValid}, nil
}

// VerifyContractAddress 验证智能合约地址格式
// 智能合约地址与普通地址格式相同（0x 开头 + 40 位十六进制），但需要通过 eth_call 验证合约是否存在
func (s *TxBuilderService) VerifyContractAddress(ctx context.Context, req *txbuilder.VerifyContractAddressRequest) (*txbuilder.VerifyContractAddressResponse, error) {
	log.Debug("VerifyContractAddress received", "trace_id", req.TraceId, "address", req.Address)

	if err := verifyAddressBasic(req.TraceId, req.Address); err != nil {
		return nil, errors.InvalidArgumentErr(err)
	}

	isValid := ethereum.ValidateContractAddress(req.Address)

	log.Info("VerifyContractAddress success!", "trace_id", req.TraceId, "contract", req.Address, "is_valid", isValid)
	return &txbuilder.VerifyContractAddressResponse{IsValid: isValid}, nil
}

// ConvertAddress 将 PEM-encoded PKIX 格式公钥转换为 Ethereum 地址
func (s *TxBuilderService) ConvertAddress(ctx context.Context, req *txbuilder.ConvertAddressRequest) (*txbuilder.ConvertAddressResponse, error) {
	log.Debug("ConvertAddress received", "trace_id", req.TraceId, "keys_count", len(req.Keys))

	if err := validateConvertAddress(req); err != nil {
		log.Warn("Invalid input parameter", "trace_id", req.TraceId, "error", err)
		return nil, errors.InvalidArgumentErr(err)
	}

	results := make([]*txbuilder.PublicKeysResponse, 0, len(req.Keys))
	for _, key := range req.Keys {
		addr, err := ethereum.PublicKeyPEMToAddress(key.PkixPubkeyPem)
		if err != nil {
			log.Error("Failed to convert public key", "trace_id", req.TraceId, "account_index", key.AccountIndex, "error", err)
			return nil, errors.InvalidArgument("pkix_pubkey_pem")
		}
		results = append(results, &txbuilder.PublicKeysResponse{
			AccountIndex: key.AccountIndex,
			Address:      addr,
		})
	}

	log.Info("ConvertAddress success!", "trace_id", req.TraceId, "success_count", len(results))
	return &txbuilder.ConvertAddressResponse{Keys: results}, nil
}

// verifyAddress 验证地址的基本校验逻辑
func verifyAddressBasic(traceID, address string) error {
	if traceID == "" {
		return errors.New("trace_id is required")
	}
	if address == "" {
		return errors.New("address is required")
	}

	return nil
}

func validateConvertAddress(req *txbuilder.ConvertAddressRequest) error {
	if req.TraceId == "" || len(req.TraceId) > 36 {
		return errors.New("trace_id is required and must be 1-36 characters")
	}
	if len(req.Keys) == 0 {
		return errors.New("keys is required")
	}
	if len(req.Keys) > 100 {
		return errors.New("keys length exceeds maximum of 100")
	}
	for _, key := range req.Keys {
		if strings.TrimSpace(key.PkixPubkeyPem) == "" {
			return errors.New("pkix_pubkey_pem is required and must not be empty")
		}
		if len(key.PkixPubkeyPem) > 4096 {
			return errors.New("pkix_pubkey_pem exceeds maximum length of 4096 characters")
		}
	}
	return nil
}
