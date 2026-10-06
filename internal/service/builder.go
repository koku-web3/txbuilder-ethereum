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
	"github.com/koku-web3/txbuilder-ethereum/internal/pkg/errors"
	"github.com/koku-web3/txbuilder-ethereum/internal/pkg/params"
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
	log.Debug("BuildSignRawData received", "trace_id", req.TraceId,
		"chain_code", req.ChainCode, "coin", req.Coin, "coin_symbol", req.CoinSymbol,
		"from_address", req.FromAddress, "to_address", req.ToAddress,
		"amount", req.Amount, "contract", req.Contract)

	if err := s.validateBuildSignRawData(req); err != nil {
		log.Warn("Invalid input parameter", "trace_id", req.TraceId, "error", err)
		return nil, errors.InvalidArgumentErr(err)
	}

	var (
		err          error
		msg, rawData string
	)

	if params.IsContractOpt(req.Contract) {
		msg, rawData, err = s.buildTokenTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount, req.Contract)
	} else {
		msg, rawData, err = s.buildBasicCoinTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount)
	}

	if len(msg) != 64 {
		log.Error("Message Hash length invalid", "trace_id", req.TraceId, "expect_length", "64", "actual_length", len(msg), "msg", msg)
		return nil, errors.Internal("length of message invalid.")
	}

	if err != nil {
		log.Error("Failed to build transaction", "trace_id", req.TraceId, "error", err)
		return nil, errors.Internal("failed to build transaction.")
	}

	log.Info("Build raw data of signing success", "trace_id", req.TraceId, "msg", msg, "raw_data", rawData)
	return &txbuilder.BuildSignRawDataResponse{Msg: msg, RawData: rawData}, nil
}

// buildBasicCoinTransaction 构建原生币（ETH）转账交易
// 使用 EIP-1559 签名，需要 chainId 防重放攻击
// 返回值：
//   - msg: EIP-1559 签名哈希（Keccak-256(RLP(nonce, gasPrice, gasLimit, to, value, chainId, 0, 0))）
//   - rawData: RLP 编码的交易数据（用于签名后组装完整签名交易）
func (s *TxBuilderService) buildBasicCoinTransaction(ctx context.Context, from, to, amount string) (string, string, error) {
	log.Debug("Building ETH transfer", "from", from, "to", to, "amount", amount)

	amountInt, err := ethereum.ParseAmount(amount)
	if err != nil {
		return "", "", err
	}

	nonce, err := s.rpc.GetTransactionCount(ctx, from)
	if err != nil {
		return "", "", errors.Wrap(err, "geth GetTransactionCount failed")
	}

	gasTipCap, err := s.rpc.SuggestGasTipCap(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get suggest tip cap")
	}

	gasFeeCap, err := s.rpc.SuggestGasFeeCap(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get suggest gas fee cap")
	}

	toAddress := common.HexToAddress(to)

	chainId := big.NewInt(int64(s.rpc.ChainID))
	// 构建 EIP-1559 DynamicFee 交易：
	// - ChainID: 链 ID（防重放攻击）
	// - Nonce: 交易计数，防止重放
	// - GasTipCap: maxPriorityFeePerGas（小费上限）
	// - GasFeeCap: maxFeePerGas（总价格上限）
	// - Gas: 21000（ETH 转账固定消耗）
	// - To: 收款地址
	// - Value: 转账金额（wei）
	// - Data: nil（原生币转账无数据）
	// - AccessList: nil（EIP-2930 访问列表，EIP-1559 可为空）
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

	// EIP-1559 签名：使用 chainId 防止跨链重放攻击
	// 签名哈希 = Keccak-256(RLP(nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit, to, value, chainId, 0, 0))
	signer := types.NewLondonSigner(chainId)
	hash := signer.Hash(tx)

	msg := hex.EncodeToString(hash.Bytes())

	rawData, err := tx.MarshalBinary()
	if err != nil {
		return "", "", fmt.Errorf("Builded tx marshalBinary failed %w", err)
	}

	log.Debug("Transaction built", "from", from, "nonce", nonce, "msg_length", len(msg))
	return msg, hex.EncodeToString(rawData), nil
}

// buildTokenTransaction 构建 ERC20 Token 转账交易
// 调用合约的 transfer 方法，data 包含 methodID + 收款地址 + 金额
// 返回值：
//   - msg: EIP-1559 签名哈希
//   - rawData: RLP 编码的交易数据
func (s *TxBuilderService) buildTokenTransaction(ctx context.Context, from, to, amount, contract string) (string, string, error) {
	log.Debug("Building ERC20 transfer", "from", from, "to", to, "amount", amount, "contract", contract)

	// amount 参数已经是链上单位（根据 proto 注释），直接解析即可
	amountInt, err := ethereum.ParseAmount(amount)
	if err != nil {
		return "", "", err
	}

	nonce, err := s.rpc.GetTransactionCount(ctx, from)
	if err != nil {
		return "", "", errors.Wrap(err, "geth GetTransactionCount failed")
	}

	gasTipCap, err := s.rpc.SuggestGasTipCap(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get suggest tip cap")
	}

	gasFeeCap, err := s.rpc.SuggestGasFeeCap(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get suggest gas fee cap")
	}

	// 构建 ERC20 transfer calldata（methodID + 收款地址 + 金额）
	data, err := ethereum.BuildERC20TransferData(to, amountInt)
	if err != nil {
		return "", "", fmt.Errorf("failed to build ERC20 transfer data: %w", err)
	}
	// Data 必须是 0x 前缀的十六进制字符串，这是以太坊 RPC 要求的格式
	dataHex := "0x" + hex.EncodeToString(data)

	// 估算合约调用所需的 gasLimit（ETH 转账固定 21000，合约调用需要估算）
	gas, err := s.rpc.EstimateGas(ctx, ethereum.CallArg{
		From: from,
		To:   contract,
		Data: dataHex,
	})
	if err != nil {
		log.Warn("Gas estimation failed, using default 100,000", "from", from, "contract", contract, "error", err)
		gas = DefaultGas // 估算失败时使用默认值 100,000 gas
	} else {
		gas = uint64(float64(gas) * 1.2) // 乘以 1.2 系数避免 gas 不足
	}

	// Token 转账：value=0（代币金额在 data 中），to=合约地址
	toAddress := common.HexToAddress(contract)

	// 构建 EIP-1559 DynamicFee 交易
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:    big.NewInt(int64(s.rpc.ChainID)),
		Nonce:      nonce,
		GasTipCap:  gasTipCap,
		GasFeeCap:  gasFeeCap,
		Gas:        gas,
		To:         &toAddress,
		Value:      big.NewInt(0), // Token 转账的 ETH value 为 0
		Data:       data,
		AccessList: nil,
	})

	// EIP-1559 签名：使用 chainId 防止跨链重放攻击
	signer := types.NewLondonSigner(big.NewInt(int64(s.rpc.ChainID)))
	hash := signer.Hash(tx)

	msg := hex.EncodeToString(hash.Bytes())

	rawData, err := tx.MarshalBinary()
	if err != nil {
		return "", "", fmt.Errorf("Builded tx marshalBinary failed %w", err)
	}

	log.Debug("Transaction built", "from", from, "nonce", nonce, "msg_length", len(msg))
	return msg, hex.EncodeToString(rawData), nil
}

func (s *TxBuilderService) validateBuildSignRawData(req *txbuilder.BuildSignRawDataRequest) error {
	if req.TraceId == "" || len(req.TraceId) > 36 {
		return errors.New("trace_id is required and must be 1-36 characters")
	}
	if req.ChainCode == "" || len(req.ChainCode) > 36 {
		return errors.New("chain_code is required and must be 1-36 characters")
	}
	if req.Coin == "" || len(req.Coin) > 36 {
		return errors.New("coin_id is required and must be 1-36 characters")
	}
	if req.CoinSymbol == "" || len(req.CoinSymbol) > 36 {
		return errors.New("coin_symbol is required and must be 1-36 characters")
	}
	if req.FromAddress == "" || len(req.FromAddress) > 256 {
		return errors.New("from_address is required and must be 1-256 characters")
	}
	if req.ToAddress == "" || len(req.ToAddress) > 256 {
		return errors.New("to_address is required and must be 1-256 characters")
	}
	if req.Amount == "" || len(req.Amount) > 64 {
		return errors.New("amount is required and must be 1-64 characters")
	}
	if !ethereum.IsPureNumber(req.Amount) {
		return errors.New("amount must be a pure number")
	}
	if !ethereum.ValidateAddress(req.FromAddress) {
		return errors.New("invalid from_address")
	}
	if !ethereum.ValidateAddress(req.ToAddress) {
		return errors.New("invalid to_address")
	}
	return nil
}
