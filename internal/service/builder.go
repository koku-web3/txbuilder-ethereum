package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/ethereum"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BuildSignRawData 构建交易签名原始数据
// 业务流程：
//  1. 验证请求参数（biz_id, 地址, 金额等）
//  2. 根据 is_basic_coin 判断是原生币（ETH）还是 Token 转账
//  3. 获取 nonce 和 gas price
//  4. 构建交易并计算签名哈希（EIP-1559）
//  5. 返回 msg（待签名哈希）和 raw_data（RLP 编码的原始交易数据）
//
// 返回值：
//   - msg: Keccak-256 哈希（Hex 编码），用于外部签名
//   - raw_data: RLP 编码的交易数据（Hex 字符串），签名后用于广播
func (s *TxBuilderService) BuildSignRawData(ctx context.Context, req *txbuilder.BuildSignRawDataRequest) (*txbuilder.BuildSignRawDataResponse, error) {
	if err := s.validateBuildSignRawData(req); err != nil {
		return nil, err
	}

	log.Info("[BuildSignRawData] Validation passed, building transaction", "trace_id", req.TraceId)

	var msg, rawData string
	var err error

	txType := "basic_coin"
	if req.Contract != "" {
		txType = "token"
	}
	log.Info("[BuildSignRawData] Building transaction", "trace_id", req.TraceId, "tx_type", txType, "from_address", req.FromAddress, "to_address", req.ToAddress, "amount", req.Amount)

	if req.Contract == "" {
		msg, rawData, err = s.buildBasicCoinTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount)
	} else {
		msg, rawData, err = s.buildTokenTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount, req.Contract)
	}

	if err != nil {
		log.Error("[BuildSignRawData] Failed to build transaction", "trace_id", req.TraceId, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to build transaction: %v", err)
	}

	log.Info("[BuildSignRawData] Transaction built", "trace_id", req.TraceId, "msg_length", len(msg), "raw_data_length", len(rawData))

	return &txbuilder.BuildSignRawDataResponse{Msg: msg, RawData: rawData}, nil
}

// buildBasicCoinTransaction 构建原生币（ETH）转账交易
// 使用 EIP-1559 签名，需要 chainId 防重放攻击
// 返回值：
//   - msg: EIP-1559 签名哈希（Keccak-256(RLP(nonce, gasPrice, gasLimit, to, value, chainId, 0, 0))）
//   - rawData: RLP 编码的交易数据（用于签名后组装完整签名交易）
func (s *TxBuilderService) buildBasicCoinTransaction(ctx context.Context, from, to, amount string) (string, string, error) {
	log.Debug("[buildBasicCoinTransaction] Building ETH transfer", "from", from, "to", to, "amount", amount)

	amountInt, err := parseAmount(amount)
	if err != nil {
		return "", "", errors.Wrap(err, "invalid amount")
	}

	nonce, err := s.rpc.GetTransactionCount(ctx, from)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get nonce")
	}

	gasTipCap, err := s.rpc.SuggestGasTipCap(ctx)
	if err != nil {
		log.Warn("[buildBasicCoinTransaction] Failed to get gas tip cap, using fallback", "error", err)
		return "", "", errors.Wrap(err, "failed to get tip cap")
	}

	gasFeeCap, err := s.rpc.SuggestGasFeeCap(ctx)
	if err != nil {
		log.Warn("[buildBasicCoinTransaction] Failed to get gas fee cap, using fallback", "error", err)
		return "", "", errors.Wrap(err, "failed to get gas fee cap")
	}

	toAddress := common.HexToAddress(to)

	chainId := big.NewInt(int64(s.rpc.ChainID))
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:    chainId,
		Nonce:      nonce,
		GasTipCap:  gasTipCap,
		GasFeeCap:  gasFeeCap,
		Gas:        BaseCoinFixGas,
		To:         &toAddress,
		Value:      amountInt,
		Data:       nil,
		AccessList: nil,
	})

	signer := types.NewLondonSigner(chainId)
	hash := signer.Hash(tx)

	msg := hex.EncodeToString(hash.Bytes())

	rawData, err := tx.MarshalBinary()
	if err != nil {
		return "", "", fmt.Errorf("[buildBasicCoinTransaction] tx marshalBinary failed %w", err)
	}

	log.Debug("[buildBasicCoinTransaction] Transaction built", "from", from, "nonce", nonce, "msg_length", len(msg))

	return msg, hex.EncodeToString(rawData), nil
}

// buildTokenTransaction 构建 ERC20 Token 转账交易
// 调用合约的 transfer 方法，data 包含 methodID + 收款地址 + 金额
// 返回值：
//   - msg: EIP-1559 签名哈希
//   - rawData: RLP 编码的交易数据
func (s *TxBuilderService) buildTokenTransaction(ctx context.Context, from, to, amount, contract string) (string, string, error) {
	log.Debug("[buildTokenTransaction] Building ERC20 transfer", "from", from, "to", to, "amount", amount, "contract", contract)

	amountInt, err := parseAmount(amount)
	if err != nil {
		return "", "", errors.Wrap(err, "invalid amount")
	}

	nonce, err := s.rpc.GetTransactionCount(ctx, from)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get nonce")
	}

	gasTipCap, err := s.rpc.SuggestGasTipCap(ctx)
	if err != nil {
		log.Warn("[buildTokenTransaction] Failed to get gas tip cap, using fallback", "error", err)
		return "", "", errors.Wrap(err, "failed to get tip cap")
	}

	gasFeeCap, err := s.rpc.SuggestGasFeeCap(ctx)
	if err != nil {
		log.Warn("[buildTokenTransaction] Failed to get gas fee cap, using fallback", "error", err)
		return "", "", errors.Wrap(err, "failed to get gas fee cap")
	}

	data := buildERC20TransferData(to, amountInt)

	gas, err := s.rpc.EstimateGas(ctx, ethereum.CallArg{
		From: from,
		To:   contract,
		Data: string(data),
	})
	if err != nil {
		log.Warn("[buildTokenTransaction] Gas estimation failed, using default 100000", "error", err)
		gas = 100000
	} else {
		gas = uint64(float64(gas) * 1.2)
	}

	toAddress := common.HexToAddress(contract)

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:    big.NewInt(int64(s.rpc.ChainID)),
		Nonce:      nonce,
		GasTipCap:  gasTipCap,
		GasFeeCap:  gasFeeCap,
		Gas:        gas,
		To:         &toAddress,
		Value:      big.NewInt(0),
		Data:       data,
		AccessList: nil,
	})

	signer := types.NewLondonSigner(big.NewInt(int64(s.rpc.ChainID)))
	hash := signer.Hash(tx)

	msg := hex.EncodeToString(hash.Bytes())

	rawData, err := tx.MarshalBinary()
	if err != nil {
		return "", "", fmt.Errorf("[buildTokenTransaction] tx marshalBinary failed %w", err)
	}

	log.Debug("[buildTokenTransaction] Transaction built", "from", from, "nonce", nonce, "msg_length", len(msg))

	return msg, hex.EncodeToString(rawData), nil
}
