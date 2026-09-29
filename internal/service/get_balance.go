package service

import (
	"context"
	"math/big"
)

// getBasicCoinBalance 获取 ETH 原生币余额
// 调用 eth_getBalance RPC 方法
func (s *TxBuilderService) getBasicCoinBalance(ctx context.Context, address string) (*big.Int, error) {
	balance, err := s.rpc.GetBalance(ctx, address)
	if err != nil {
		return nil, err
	}
	return balance, nil
}

// getTokenBalance 获取 ERC20 Token 余额
// 通过 eth_call 调用合约的 balanceOf 方法（methodID: 0x70a08231）
// 返回的余额是原始数值（即单位 WEI）
func (s *TxBuilderService) getTokenBalance(ctx context.Context, owner, contract string) (*big.Int, error) {
	return s.rpc.GetTokenBalance(ctx, owner, contract)
}
