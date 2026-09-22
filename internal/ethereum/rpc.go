package ethereum

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	log "github.com/koku-web3/logko"
	"github.com/koku-web3/txbuilder-ethereum/internal/config"
	"github.com/pkg/errors"
)

type RPCClient struct {
	RpcURL  string
	ChainID uint64
	client  *http.Client
}

func NewRPCClient() *RPCClient {
	cfg := config.GetConfig()
	return &RPCClient{
		RpcURL:  strings.TrimSuffix(cfg.Chain.RPCURL, "/"),
		ChainID: cfg.Chain.ChainID,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *RPCClient) call(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		log.Error("[RPCClient.call] Failed to marshal request", "method", method, "error", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.RpcURL, bytes.NewReader(body))
	if err != nil {
		log.Error("[RPCClient.call] Failed to create request", "url", c.RpcURL, "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Error("[RPCClient.call] Failed to do request", "url", c.RpcURL, "method", method, "error", err)
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Error("[RPCClient.call] RPC call failed", "url", c.RpcURL, "method", method, "status_code", resp.StatusCode, "response_body", string(respBody))
		return nil, fmt.Errorf("rpc call failed with status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("[RPCClient.call] Failed to read response body", "url", c.RpcURL, "method", method, "error", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	type jsonRPCError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	type jsonRPCResponse struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *jsonRPCError   `json:"error,omitempty"`
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		log.Error("[RPCClient.call] Failed to unmarshal JSON-RPC response", "url", c.RpcURL, "method", method, "error", err)
		return nil, fmt.Errorf("failed to unmarshal JSON-RPC response: %w", err)
	}

	if rpcResp.Error != nil {
		log.Error("[RPCClient.call] JSON-RPC error", "url", c.RpcURL, "method", method, "error_code", rpcResp.Error.Code, "error_message", rpcResp.Error.Message)
		return nil, fmt.Errorf("eth rpc error: code=%d msg=%s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// BlockHeader 区块头信息（用于 EIP-1559 baseFee 计算）
type BlockHeader struct {
	GasLimit      uint64
	GasUsed       uint64
	BaseFeePerGas *big.Int
}

func (c *RPCClient) ChainId(ctx context.Context) (uint64, error) {
	result, err := c.call(ctx, "eth_chainId", []interface{}{})
	if err != nil {
		log.Error("[RPCClient.ChainId] Failed to get chain ID", "error", err)
		return 0, err
	}

	var chainIDHex string
	if err := json.Unmarshal(result, &chainIDHex); err != nil {
		log.Error("[RPCClient.ChainId] Failed to unmarshal chain ID", "result", string(result), "error", err)
		return 0, fmt.Errorf("failed to unmarshal chain ID: %w", err)
	}

	chainID, err := parseHexToUint64(chainIDHex)
	if err != nil {
		log.Error("[RPCClient.ChainId] Failed to parse chain ID", "chain_id_hex", chainIDHex, "error", err)
		return 0, err
	}

	return chainID, nil
}

func (c *RPCClient) BlockNumber(ctx context.Context) (string, error) {
	result, err := c.call(ctx, "eth_blockNumber", []interface{}{})
	if err != nil {
		log.Error("[RPCClient.BlockNumber] Failed to get block number", "error", err)
		return "", err
	}

	var blockNumber string
	if err := json.Unmarshal(result, &blockNumber); err != nil {
		log.Error("[RPCClient.BlockNumber] Failed to unmarshal block number", "result", string(result), "error", err)
		return "", fmt.Errorf("failed to unmarshal block number: %w", err)
	}

	return blockNumber, nil
}

// GetBlockByNumber 获取指定区块的区块头信息（用于 EIP-1559 baseFee 计算）
// tag 支持 "latest"、"earliest"、"pending" 或具体区块号
func (c *RPCClient) GetBlockByNumber(ctx context.Context, tag string) (*BlockHeader, error) {
	params := []interface{}{tag, false} // false = 返回完整区块对象（非完整交易）
	result, err := c.call(ctx, "eth_getBlockByNumber", params)
	if err != nil {
		log.Error("[RPCClient.GetBlockByNumber] Failed to get block", "tag", tag, "error", err)
		return nil, err
	}

	type blockResponse struct {
		GasLimit      string `json:"gasLimit"`
		GasUsed       string `json:"gasUsed"`
		BaseFeePerGas string `json:"baseFeePerGas"`
	}

	var block blockResponse
	if err := json.Unmarshal(result, &block); err != nil {
		log.Error("[RPCClient.GetBlockByNumber] Failed to unmarshal block", "result", string(result), "error", err)
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	gasLimit, err := parseHexToUint64(block.GasLimit)
	if err != nil {
		log.Error("[RPCClient.GetBlockByNumber] Failed to parse gasLimit", "gasLimit", block.GasLimit, "error", err)
		return nil, fmt.Errorf("failed to parse gasLimit: %w", err)
	}

	gasUsed, err := parseHexToUint64(block.GasUsed)
	if err != nil {
		log.Error("[RPCClient.GetBlockByNumber] Failed to parse gasUsed", "gasUsed", block.GasUsed, "error", err)
		return nil, fmt.Errorf("failed to parse gasUsed: %w", err)
	}

	baseFeePerGas := new(big.Int)
	if block.BaseFeePerGas != "" {
		baseFeePerGasHex := strings.TrimPrefix(block.BaseFeePerGas, "0x")
		baseFeePerGas.SetString(baseFeePerGasHex, 16)
	}

	return &BlockHeader{GasLimit: gasLimit, GasUsed: gasUsed, BaseFeePerGas: baseFeePerGas}, nil
}

// MaxPriorityFeePerGas 获取当前网络建议的小费（maxPriorityFeePerGas）
// 等同于 eth_maxPriorityFeePerGas RPC 方法
func (c *RPCClient) MaxPriorityFeePerGas(ctx context.Context) (*big.Int, error) {
	result, err := c.call(ctx, "eth_maxPriorityFeePerGas", []interface{}{})
	if err != nil {
		log.Error("[RPCClient.MaxPriorityFeePerGas] Failed to get max priority fee", "error", err)
		return nil, err
	}

	var feeHex string
	if err := json.Unmarshal(result, &feeHex); err != nil {
		log.Error("[RPCClient.MaxPriorityFeePerGas] Failed to unmarshal fee", "result", string(result), "error", err)
		return nil, fmt.Errorf("failed to unmarshal maxPriorityFeePerGas: %w", err)
	}

	fee := new(big.Int)
	feeHex = strings.TrimPrefix(feeHex, "0x")
	fee.SetString(feeHex, 16)

	return fee, nil
}

// SuggestGasTipCap 获取 GasTipCap (maxPriorityFeePerGas)
// 即用户愿意支付给矿工/验证者的最大小费
// 等同于调用 eth_maxPriorityFeePerGas
func (c *RPCClient) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	tipCap, err := c.MaxPriorityFeePerGas(ctx)
	if err != nil {
		log.Error("[RPCClient.SuggestGasTipCap] Failed to get tip cap", "error", err)
		return nil, errors.Wrap(err, "failed to get gas tip cap")
	}

	return tipCap, nil
}

// SuggestGasFeeCap 获取 GasFeeCap (maxFeePerGas)
// 计算公式: maxFeePerGas = 2 * baseFee + maxPriorityFeePerGas
// 参考 EIP-1559 文档中的计算方式
func (c *RPCClient) SuggestGasFeeCap(ctx context.Context) (*big.Int, error) {
	// Step 1: 获取最新区块，计算下一区块的 baseFee
	block, err := c.GetBlockByNumber(ctx, "latest")
	if err != nil {
		log.Error("[RPCClient.SuggestGasFeeCap] Failed to get block", "error", err)
		return nil, errors.Wrap(err, "failed to get latest block for baseFee")
	}

	// 检查是否为 EIP-1559 链（baseFeePerGas 必须大于 0）
	if block.BaseFeePerGas.Cmp(big.NewInt(0)) == 0 {
		log.Error("[RPCClient.SuggestGasFeeCap] Chain does not support EIP-1559")
		return nil, errors.New("chain does not support EIP-1559")
	}

	// Step 2: EIP-1559 baseFee 计算公式
	parentGasTarget := block.GasLimit / 2
	var baseFee *big.Int

	if block.GasUsed == parentGasTarget {
		baseFee = block.BaseFeePerGas
	} else if block.GasUsed > parentGasTarget {
		// gas 用多了，费用上涨
		diff := block.GasUsed - parentGasTarget
		num := new(big.Int).Mul(block.BaseFeePerGas, new(big.Int).SetUint64(diff))
		num.Div(num, new(big.Int).SetUint64(parentGasTarget))
		num.Div(num, big.NewInt(8))
		if num.Cmp(big.NewInt(1)) < 0 {
			num = big.NewInt(1)
		}
		baseFee = new(big.Int).Add(block.BaseFeePerGas, num)
	} else {
		// gas 用少了，费用降低
		diff := parentGasTarget - block.GasUsed
		num := new(big.Int).Mul(block.BaseFeePerGas, new(big.Int).SetUint64(diff))
		num.Div(num, new(big.Int).SetUint64(parentGasTarget))
		num.Div(num, big.NewInt(8))
		baseFee = new(big.Int).Sub(block.BaseFeePerGas, num)
		if baseFee.Cmp(big.NewInt(0)) < 0 {
			baseFee = big.NewInt(0)
		}
	}

	// Step 3: 获取 maxPriorityFeePerGas（小费）
	maxPriorityFee, err := c.MaxPriorityFeePerGas(ctx)
	if err != nil {
		log.Warn("[RPCClient.SuggestGasFeeCap] Failed to get maxPriorityFeePerGas, using fallback 2 gwei", "error", err)
		maxPriorityFee = big.NewInt(2000000000) // 2 gwei
	}

	// Step 4: 计算 maxFeePerGas = 2 * baseFee + maxPriorityFeePerGas
	feeCap := new(big.Int).Mul(big.NewInt(2), baseFee)
	feeCap.Add(feeCap, maxPriorityFee)

	return feeCap, nil
}

func (c *RPCClient) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	result, err := c.call(ctx, "eth_getBalance", []interface{}{address, "latest"})
	if err != nil {
		log.Error("[RPCClient.GetBalance] Failed to get balance", "address", address, "error", err)
		return nil, err
	}

	var balanceHex string
	if err := json.Unmarshal(result, &balanceHex); err != nil {
		log.Error("[RPCClient.GetBalance] Failed to unmarshal balance", "address", address, "result", string(result), "error", err)
		return nil, fmt.Errorf("failed to unmarshal balance: %w", err)
	}

	balance := new(big.Int)
	balanceHex = strings.TrimPrefix(balanceHex, "0x")
	balance.SetString(balanceHex, 16)

	return balance, nil
}

func (c *RPCClient) GetTransactionCount(ctx context.Context, address string) (uint64, error) {
	result, err := c.call(ctx, "eth_getTransactionCount", []interface{}{address, "pending"})
	if err != nil {
		log.Error("[RPCClient.GetTransactionCount] Failed to get transaction count", "address", address, "error", err)
		return 0, err
	}

	var nonceHex string
	if err := json.Unmarshal(result, &nonceHex); err != nil {
		log.Error("[RPCClient.GetTransactionCount] Failed to unmarshal nonce", "address", address, "result", string(result), "error", err)
		return 0, fmt.Errorf("failed to unmarshal nonce: %w", err)
	}

	nonce, err := parseHexToUint64(nonceHex)
	if err != nil {
		log.Error("[RPCClient.GetTransactionCount] Failed to parse nonce", "address", address, "nonce_hex", nonceHex, "error", err)
		return 0, err
	}

	return nonce, nil
}

func (c *RPCClient) GasPrice(ctx context.Context) (*big.Int, error) {
	result, err := c.call(ctx, "eth_gasPrice", []interface{}{})
	if err != nil {
		log.Error("[RPCClient.GasPrice] Failed to get gas price", "error", err)
		return nil, err
	}

	var gasPriceHex string
	if err := json.Unmarshal(result, &gasPriceHex); err != nil {
		log.Error("[RPCClient.GasPrice] Failed to unmarshal gas price", "result", string(result), "error", err)
		return nil, fmt.Errorf("failed to unmarshal gas price: %w", err)
	}

	gasPrice := new(big.Int)
	gasPriceHex = strings.TrimPrefix(gasPriceHex, "0x")
	gasPrice.SetString(gasPriceHex, 16)

	return gasPrice, nil
}

func (c *RPCClient) GetCode(ctx context.Context, address string) (string, error) {
	result, err := c.call(ctx, "eth_getCode", []interface{}{address, "latest"})
	if err != nil {
		log.Error("[RPCClient.GetCode] Failed to get code", "address", address, "error", err)
		return "", err
	}

	var code string
	if err := json.Unmarshal(result, &code); err != nil {
		log.Error("[RPCClient.GetCode] Failed to unmarshal code", "address", address, "result", string(result), "error", err)
		return "", fmt.Errorf("failed to unmarshal code: %w", err)
	}

	return code, nil
}

type CallArg struct {
	From  string `json:"from,omitempty"`
	To    string `json:"to"`
	Value string `json:"value,omitempty"`
	Data  string `json:"data,omitempty"`
}

func (c *RPCClient) EstimateGas(ctx context.Context, callObj CallArg) (uint64, error) {
	result, err := c.call(ctx, "eth_estimateGas", []interface{}{callObj})
	if err != nil {
		log.Error("[RPCClient.EstimateGas] Failed to estimate gas", "call_obj", callObj, "error", err)
		return 0, err
	}

	var gasHex string
	if err := json.Unmarshal(result, &gasHex); err != nil {
		log.Error("[RPCClient.EstimateGas] Failed to unmarshal gas", "result", string(result), "error", err)
		return 0, fmt.Errorf("failed to unmarshal gas: %w", err)
	}

	gas, err := parseHexToUint64(gasHex)
	if err != nil {
		log.Error("[RPCClient.EstimateGas] Failed to parse gas", "gas_hex", gasHex, "error", err)
		return 0, err
	}

	return gas, nil
}

func (c *RPCClient) Call(ctx context.Context, callObj CallArg) (string, error) {
	result, err := c.call(ctx, "eth_call", []interface{}{callObj, "latest"})
	if err != nil {
		log.Error("[RPCClient.Call] Failed to call contract", "call_obj", callObj, "error", err)
		return "", err
	}

	var resultStr string
	if err := json.Unmarshal(result, &resultStr); err != nil {
		log.Error("[RPCClient.Call] Failed to unmarshal result", "result", string(result), "error", err)
		return "", fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return resultStr, nil
}

func (c *RPCClient) BroadcastRawTransaction(ctx context.Context, rawHex string) (string, error) {
	// 广播的数据需要以 0x 开头，如果rawHex非 0x 开头，补上
	if !strings.HasPrefix(rawHex, "0x") {
		rawHex = "0x" + rawHex
	}

	result, err := c.call(ctx, "eth_sendRawTransaction", []interface{}{rawHex})
	if err != nil {
		log.Error("[RPCClient.BroadcastRawTransaction] Broadcast failed", "error", err)
		return "", err
	}

	var txHash string
	if err := json.Unmarshal(result, &txHash); err != nil {
		log.Error("[RPCClient.BroadcastRawTransaction] Failed to unmarshal tx hash", "result", string(result), "error", err)
		return "", fmt.Errorf("failed to unmarshal tx hash: %w", err)
	}

	return txHash, nil
}

func parseHexToUint64(hexStr string) (uint64, error) {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	var result uint64
	for _, c := range hexStr {
		var val uint64
		switch {
		case c >= '0' && c <= '9':
			val = uint64(c - '0')
		case c >= 'a' && c <= 'f':
			val = uint64(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			val = uint64(c - 'A' + 10)
		default:
			return 0, fmt.Errorf("invalid hex character: %c", c)
		}
		result = result<<4 | val
	}
	return result, nil
}

func HexToBytes(hexStr string) ([]byte, error) {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	if len(hexStr)%2 != 0 {
		return nil, fmt.Errorf("odd length hex string")
	}
	return hex.DecodeString(hexStr)
}
