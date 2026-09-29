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
	log.Debug("Calling geth ChainId")

	signStart := time.Now()
	result, err := c.call(ctx, "eth_chainId", []interface{}{})
	cost := time.Since(signStart)

	if err != nil {
		return 0, err
	}

	var chainIDHex string
	if err := json.Unmarshal(result, &chainIDHex); err != nil {
		return 0, fmt.Errorf("failed to unmarshal chain ID: %w", err)
	}

	chainID, err := parseHexToUint64(chainIDHex)
	if err != nil {
		return 0, fmt.Errorf("failed to parse chain ID(hex=%s): %w", chainIDHex, err)
	}
	log.Debug("Call geth ChainId success!", "chain_id", chainID, "time_cost_ms", cost.Microseconds())
	return chainID, nil
}

func (c *RPCClient) BlockNumber(ctx context.Context) (string, error) {
	log.Debug("Calling geth BlockNumber")

	signStart := time.Now()
	result, err := c.call(ctx, "eth_blockNumber", []interface{}{})
	cost := time.Since(signStart)

	if err != nil {
		return "", err
	}

	var blockNumber string
	if err := json.Unmarshal(result, &blockNumber); err != nil {
		return "", fmt.Errorf("failed to unmarshal block number: %w", err)
	}
	log.Debug("Call geth BlockNumber success!", "block_number", blockNumber, "time_cost_ms", cost.Microseconds())
	return blockNumber, nil
}

// GetBlockByNumber 获取指定区块的区块头信息（用于 EIP-1559 baseFee 计算）
// tag 支持 "latest"、"earliest"、"pending" 或具体区块号
func (c *RPCClient) GetBlockByNumber(ctx context.Context, tag string) (*BlockHeader, error) {
	log.Debug("Calling geth GetBlockByNumber", "tag", tag)

	signStart := time.Now()
	params := []interface{}{tag, false} // false = 返回完整区块对象（非完整交易）
	result, err := c.call(ctx, "eth_getBlockByNumber", params)
	cost := time.Since(signStart)

	if err != nil {
		return nil, err
	}

	type blockResponse struct {
		GasLimit      string `json:"gasLimit"`
		GasUsed       string `json:"gasUsed"`
		BaseFeePerGas string `json:"baseFeePerGas"`
	}

	var block blockResponse
	if err := json.Unmarshal(result, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	gasLimit, err := parseHexToUint64(block.GasLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gasLimit(hex=%s): %w", block.GasLimit, err)
	}

	gasUsed, err := parseHexToUint64(block.GasUsed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gasUsed(hex=%s): %w", block.GasUsed, err)
	}

	baseFeePerGas := new(big.Int)
	if block.BaseFeePerGas != "" {
		baseFeePerGasHex := strings.TrimPrefix(block.BaseFeePerGas, "0x")
		baseFeePerGas.SetString(baseFeePerGasHex, 16)
	}
	log.Debug("Call geth GetBlockByNumber success!", "tag", tag, "time_cost_ms", cost.Microseconds())
	return &BlockHeader{GasLimit: gasLimit, GasUsed: gasUsed, BaseFeePerGas: baseFeePerGas}, nil
}

// MaxPriorityFeePerGas 获取当前网络建议的小费（maxPriorityFeePerGas）
// 等同于 eth_maxPriorityFeePerGas RPC 方法
func (c *RPCClient) MaxPriorityFeePerGas(ctx context.Context) (*big.Int, error) {
	log.Debug("Calling geth MaxPriorityFeePerGas")

	signStart := time.Now()
	result, err := c.call(ctx, "eth_maxPriorityFeePerGas", []interface{}{})
	cost := time.Since(signStart)

	if err != nil {
		return nil, err
	}

	var feeHex string
	if err := json.Unmarshal(result, &feeHex); err != nil {
		return nil, fmt.Errorf("failed to unmarshal maxPriorityFeePerGas: %w", err)
	}

	fee := new(big.Int)
	feeHex = strings.TrimPrefix(feeHex, "0x")
	fee.SetString(feeHex, 16)
	log.Debug("Call geth MaxPriorityFeePerGas success!", "fee", fee.String(), "time_cost_ms", cost.Microseconds())
	return fee, nil
}

// SuggestGasTipCap 获取 GasTipCap (maxPriorityFeePerGas)
// 即用户愿意支付给矿工/验证者的最大小费
// 等同于调用 eth_maxPriorityFeePerGas
func (c *RPCClient) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return c.MaxPriorityFeePerGas(ctx)
}

// SuggestGasFeeCap 获取 GasFeeCap (maxFeePerGas)
// 计算公式: maxFeePerGas = 2 * baseFee + maxPriorityFeePerGas
// 参考 EIP-1559 文档中的计算方式
func (c *RPCClient) SuggestGasFeeCap(ctx context.Context) (*big.Int, error) {
	// Step 1: 获取最新区块，计算下一区块的 baseFee
	block, err := c.GetBlockByNumber(ctx, "latest")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get latest block for baseFee")
	}

	// 检查是否为 EIP-1559 链（baseFeePerGas 必须大于 0）
	if block.BaseFeePerGas.Cmp(big.NewInt(0)) == 0 {
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
		log.Warn("Failed to get maxPriorityFeePerGas, using fallback 2 gwei", "error", err)
		maxPriorityFee = big.NewInt(2000000000) // 2 gwei
	}

	// Step 4: 计算 maxFeePerGas = 2 * baseFee + maxPriorityFeePerGas
	feeCap := new(big.Int).Mul(big.NewInt(2), baseFee)
	feeCap.Add(feeCap, maxPriorityFee)

	return feeCap, nil
}

func (c *RPCClient) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	log.Debug("Calling geth GetBalance", "address", address)

	signStart := time.Now()
	result, err := c.call(ctx, "eth_getBalance", []interface{}{address, "latest"})
	cost := time.Since(signStart)

	if err != nil {
		return nil, err
	}

	var balanceHex string
	if err := json.Unmarshal(result, &balanceHex); err != nil {
		return nil, fmt.Errorf("failed to unmarshal balance: %w", err)
	}

	balance := new(big.Int)
	balanceHex = strings.TrimPrefix(balanceHex, "0x")
	balance.SetString(balanceHex, 16)
	log.Debug("Call geth GetBalance success!", "address", address, "balance", balance.String(), "time_cost_ms", cost.Microseconds())
	return balance, nil
}

func (c *RPCClient) GetTokenBalance(ctx context.Context, owner, contract string) (*big.Int, error) {
	log.Debug("Calling geth getTokenBalance", "owner", owner, "contract", contract)

	methodID := "0x70a08231"
	paddedOwner, err := PadAddressTo32Bytes(owner)
	if err != nil {
		return nil, fmt.Errorf("invalid owner address: %w", err)
	}
	data := methodID + paddedOwner

	signStart := time.Now()
	res, err := c.call(ctx, "eth_call", []interface{}{CallArg{To: contract, Data: data}, "latest"})
	cost := time.Since(signStart)

	if err != nil {
		return nil, err
	}

	var balanceHexWith0x string
	if err := json.Unmarshal(res, &balanceHexWith0x); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	balanceHex := strings.TrimPrefix(balanceHexWith0x, "0x")
	if !isHexString(balanceHex) {
		return nil, fmt.Errorf("invalid balanceOf response: non-hex payload (owner=%s, contract=%s)", owner, contract)
	}
	if len(balanceHex) == 0 {
		// 合约代码ERC20标准 balanceOf 或 eth_call 返回空 data，不能当作余额 0
		return nil, fmt.Errorf("empty balanceOf response: contract may not implement balanceOf (owner=%s, contract=%s)", owner, contract)
	}
	// ERC20 balanceOf 按 ABI 返回 uint256，固定 32 字节（64 个 hex 字符）。
	if len(balanceHex) != 64 {
		return nil, fmt.Errorf("invalid balanceOf response: got %d hex chars, want 64 (owner=%s, contract=%s)", len(balanceHex), owner, contract)
	}

	balance, ok := new(big.Int).SetString(balanceHex, 16)
	if !ok {
		return nil, fmt.Errorf("failed to parse balance hex (owner=%s, contract=%s)", owner, contract)
	}
	log.Debug("Call geth GetTokenBalance success!", "owner", owner, "contract", contract, "balance", balance.String(), "time_cost_ms", cost.Microseconds())
	return balance, nil
}

func (c *RPCClient) GetTransactionCount(ctx context.Context, address string) (uint64, error) {
	log.Debug("Calling geth GetTransactionCount", "address", address)

	signStart := time.Now()
	result, err := c.call(ctx, "eth_getTransactionCount", []interface{}{address, "pending"})
	cost := time.Since(signStart)

	if err != nil {
		return 0, err
	}

	var nonceHex string
	if err := json.Unmarshal(result, &nonceHex); err != nil {
		return 0, fmt.Errorf("failed to unmarshal nonce: %w", err)
	}

	nonce, err := parseHexToUint64(nonceHex)
	if err != nil {
		return 0, fmt.Errorf("failed to parse nonce(hex=%s): %w", nonceHex, err)
	}
	log.Debug("Call geth GetTransactionCount success!", "address", address, "time_cost_ms", cost.Microseconds())
	return nonce, nil
}

func (c *RPCClient) GasPrice(ctx context.Context) (*big.Int, error) {
	log.Debug("Calling geth GasPrice")

	signStart := time.Now()
	result, err := c.call(ctx, "eth_gasPrice", []interface{}{})
	cost := time.Since(signStart)

	if err != nil {
		return nil, err
	}

	var gasPriceHex string
	if err := json.Unmarshal(result, &gasPriceHex); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gas price: %w", err)
	}

	gasPrice := new(big.Int)
	gasPriceHex = strings.TrimPrefix(gasPriceHex, "0x")
	gasPrice.SetString(gasPriceHex, 16)
	log.Debug("Call geth GasPrice success!", "gas_price", gasPrice.String(), "time_cost_ms", cost.Microseconds())
	return gasPrice, nil
}

func (c *RPCClient) GetCode(ctx context.Context, address string) (string, error) {
	log.Debug("Calling geth GetCode", "address", address)

	signStart := time.Now()
	result, err := c.call(ctx, "eth_getCode", []interface{}{address, "latest"})
	cost := time.Since(signStart)

	if err != nil {
		return "", err
	}

	var code string
	if err := json.Unmarshal(result, &code); err != nil {
		return "", fmt.Errorf("failed to unmarshal code: %w", err)
	}
	log.Debug("Call geth GetCode success!", "address", address, "code_length", len(code), "time_cost_ms", cost.Microseconds())
	return code, nil
}

type CallArg struct {
	From  string `json:"from,omitempty"`
	To    string `json:"to"`
	Value string `json:"value,omitempty"`
	Data  string `json:"data,omitempty"`
}

func (c *RPCClient) EstimateGas(ctx context.Context, callObj CallArg) (uint64, error) {
	log.Debug("Calling geth EstimateGas", "call_obj", callObj)

	signStart := time.Now()
	result, err := c.call(ctx, "eth_estimateGas", []interface{}{callObj})
	cost := time.Since(signStart)

	if err != nil {
		return 0, err
	}

	var gasHex string
	if err := json.Unmarshal(result, &gasHex); err != nil {
		return 0, fmt.Errorf("failed to unmarshal gas: %w", err)
	}

	gas, err := parseHexToUint64(gasHex)
	if err != nil {
		return 0, fmt.Errorf("failed to parse gas(hex=%s): %w", gasHex, err)
	}
	log.Debug("Call geth EstimateGas success!", "gas", gas, "time_cost_ms", cost.Microseconds())
	return gas, nil
}

// func (c *RPCClient) Call(ctx context.Context, callObj CallArg) (string, error) {
// 	log.Debug("Calling geth Call", "call_obj", callObj)

// 	signStart := time.Now()
// 	result, err := c.call(ctx, "eth_call", []interface{}{callObj, "latest"})
// 	cost := time.Since(signStart)

// 	if err != nil {
// 		return "", err
// 	}

// 	var resultStr string
// 	if err := json.Unmarshal(result, &resultStr); err != nil {
// 		return "", fmt.Errorf("failed to unmarshal result: %w", err)
// 	}
// 	log.Debug("Call geth Call success!", "call_obj", callObj, "time_cost_ms", cost.Microseconds())
// 	return resultStr, nil
// }

func (c *RPCClient) BroadcastRawTransaction(ctx context.Context, rawHex string) (string, error) {
	log.Debug("Calling geth BroadcastRawTransaction")

	// 广播的数据需要以 0x 开头，如果rawHex非 0x 开头，补上
	if !strings.HasPrefix(rawHex, "0x") {
		rawHex = "0x" + rawHex
	}

	signStart := time.Now()
	result, err := c.call(ctx, "eth_sendRawTransaction", []interface{}{rawHex})
	cost := time.Since(signStart)

	if err != nil {
		return "", err
	}

	var txHash string
	if err := json.Unmarshal(result, &txHash); err != nil {
		return "", fmt.Errorf("failed to unmarshal tx hash: %w", err)
	}
	log.Debug("Call geth BroadcastRawTransaction success!", "tx_hash", txHash, "time_cost_ms", cost.Microseconds())
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
