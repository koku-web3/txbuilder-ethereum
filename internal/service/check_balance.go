package service

import (
	"context"
	"encoding/hex"

	"math/big"

	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/ethereum"
	"github.com/koku-web3/txbuilder-ethereum/internal/pkg/errors"
	"github.com/koku-web3/txbuilder-ethereum/internal/pkg/params"
)

// CheckSufficientBalance 检查地址余额是否足够
// - is_basic_coin=true: 检查 ETH 余额是否 >= 转账金额
// - is_basic_coin=false: 先检查 ETH 余额是否足够支付 Gas，再检查 Token 余额是否 >= 转账金额
//
// 注意：根据 proto 定义，amount 参数已经是链上最小单位（如以太坊是 wei），
//
//	因此 balanceOf 返回的链上单位可直接与 amount 比较，无需 decimals 转换
func (s *TxBuilderService) CheckSufficientBalance(ctx context.Context, req *txbuilder.CheckSufficientBalanceRequest) (*txbuilder.CheckSufficientBalanceResponse, error) {
	log.Debug("CheckSufficientBalance received", "trace_id", req.TraceId,
		"chain_code", req.ChainCode, "coin", req.Coin,
		"from_address", req.FromAddress, "amount", req.Amount, "contract", req.Contract)

	if err := s.validateCheckSufficientBalance(req); err != nil {
		log.Warn("Invalid input parameter", "trace_id", req.TraceId, "error", err)
		return nil, errors.InvalidArgumentErr(err)
	}

	isContract := params.IsContractOpt(req.Contract)
	amountInt, err := ethereum.ParseAmount(req.Amount)

	if err != nil {
		log.Warn("Invalid amount format", "trace_id", req.TraceId, "amount", req.Amount, "error", err)
		return nil, errors.InvalidArgument("amount")
	}

	balance, err := s.getBasicCoinBalance(ctx, req.FromAddress)
	if err != nil {
		log.Error("Failed to get balance", "trace_id", req.TraceId, "from_address", req.FromAddress, "error", err)
		return nil, errors.Internal()
	}

	var calldata string
	if isContract {
		// 用于模拟执行 ERC20 交易，计算它的手续费
		// 任意一个地址即可
		mockAddr := req.FromAddress
		data, err := ethereum.BuildERC20TransferData(mockAddr, amountInt)
		if err != nil {
			log.Warn("Failed to build ERC20 transfer data", "trace_id", req.TraceId, "error", err)
			return nil, errors.InvalidArgument("contract")
		}
		calldata = hex.EncodeToString(data)
	}

	fee, err := s.getEip1559TxFee(ctx, isContract, req.FromAddress, req.Contract, calldata)
	if err != nil {
		log.Error("Failed to get EIP-1559 tx fee", "trace_id", req.TraceId, "error", err)
		return nil, errors.Internal()
	}

	if !isContract {
		// 原生币转账：余额需要 >= 金额 + 手续费
		isCoinSufficient := balance.Cmp(new(big.Int).Add(amountInt, fee)) >= 0
		log.Info("CheckSufficientBalance success", "trace_id", req.TraceId, "balance", balance.String(), "required", new(big.Int).Add(amountInt, fee).String(), "is_coin_sufficient", isCoinSufficient)
		return &txbuilder.CheckSufficientBalanceResponse{IsCoinSufficient: isCoinSufficient}, nil
	}

	if !ethereum.ValidateContractAddress(req.Contract) {
		return nil, errors.InvalidArgument("contract")
	}

	// Token 转账：只需检查 ETH 余额是否足够支付 Gas
	isCoinSufficient := balance.Cmp(fee) >= 0

	tokenBalance, err := s.getTokenBalance(ctx, req.FromAddress, req.Contract)
	if err != nil {
		log.Error("Failed to get token balance", "trace_id", req.TraceId, "from_address", req.FromAddress, "contract", req.Contract, "error", err)
		return nil, errors.Internal()
	}

	// amount 已是链上最小单位，与 balanceOf 返回值直接比较
	isTokenSufficient := tokenBalance.Cmp(amountInt) >= 0

	log.Info("CheckSufficientBalance Token balance success", "trace_id", req.TraceId, "token_balance", tokenBalance.String(), "required_amount", amountInt.String(), "is_coin_sufficient", isCoinSufficient, "is_token_sufficient", isTokenSufficient)

	return &txbuilder.CheckSufficientBalanceResponse{IsCoinSufficient: isCoinSufficient, IsTokenSufficient: isTokenSufficient}, nil
}

func (*TxBuilderService) validateCheckSufficientBalance(req *txbuilder.CheckSufficientBalanceRequest) error {
	if req.TraceId == "" {
		return errors.New("trace_id is required")
	}
	if req.ChainCode == "" || len(req.ChainCode) > 36 {
		return errors.New("chain_code is required and must be 1-36 characters")
	}
	if req.Coin == "" || len(req.Coin) > 36 {
		return errors.New("coin_id is required and must be 1-36 characters")
	}
	if req.FromAddress == "" || len(req.FromAddress) > 256 {
		return errors.New("from_address is required and must be 1-256 characters")
	}
	if req.Amount == "" || len(req.Amount) > 64 {
		return errors.New("amount is required and must be 1-64 characters")
	}
	return nil
}

// getEip1559TxFee 返回预估的转账总手续费（单位：wei）
// - 获取最新区块的 baseFeePerGas，按 EIP-1559 公式计算下一区块 baseFee
// - 获取 maxPriorityFeePerGas（小费）
// - 计算 perGasFee = 2 * baseFee + maxPriorityFeePerGas
// - 原生币转账 gas = 21000；合约转账使用 EstimateGas * 1.2
// - 最终返回 perGasFee * gas，即预估总手续费
func (s *TxBuilderService) getEip1559TxFee(ctx context.Context, isContract bool, from, to, data string) (*big.Int, error) {
	gasFeeCap, err := s.rpc.SuggestGasFeeCap(ctx)
	if err != nil {
		return nil, err
	}

	var gas uint64
	if isContract {
		estimated, err := s.rpc.EstimateGas(ctx, ethereum.CallArg{From: from, To: to, Data: data})
		if err != nil {
			log.Warn("Gas estimation failed, using default 100,000", "from", from, "contract", to, "error", err)
			gas = DefaultGas
		} else {
			gas = uint64(float64(estimated) * 1.2)
		}
	} else {
		gas = BaseCoinFixGas
	}

	totalFee := new(big.Int).Mul(gasFeeCap, new(big.Int).SetUint64(gas))
	return totalFee, nil
}
