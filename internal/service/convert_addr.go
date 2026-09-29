package service

import (
	"context"

	"strings"

	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/ethereum"
	"github.com/koku-web3/txbuilder-ethereum/internal/pkg/errors"
)

// ConvertAddress 将 PEM-encoded PKIX 格式公钥转换为 Ethereum 地址
func (s *TxBuilderService) ConvertAddress(ctx context.Context, req *txbuilder.ConvertAddressRequest) (*txbuilder.ConvertAddressResponse, error) {
	log.Debug("ConvertAddress received", "trace_id", req.TraceId, "keys_count", len(req.Keys))

	if err := s.validateConvertAddress(req); err != nil {
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

func (*TxBuilderService) validateConvertAddress(req *txbuilder.ConvertAddressRequest) error {
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
