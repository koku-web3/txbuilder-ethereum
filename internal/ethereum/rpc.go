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
	id := 1
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      id,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		log.Error("[RPCClient.call] failed to marshal request",
			"method", method,
			"error", err,
		)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.RpcURL

	log.Debug("[RPCClient.call] preparing request",
		"url", url,
		"method", method,
		"params", params,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Error("[RPCClient.call] failed to create request",
			"url", url,
			"error", err,
		)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Error("[RPCClient.call] failed to do request",
			"url", url,
			"method", method,
			"error", err,
		)
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	log.Debug("[RPCClient.call] received HTTP response",
		"url", url,
		"method", method,
		"status_code", resp.StatusCode,
		"status", resp.Status,
	)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("[RPCClient.call] failed to read response body",
			"url", url,
			"method", method,
			"error", err,
		)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Debug("[RPCClient.call] response body received",
		"url", url,
		"method", method,
		"status_code", resp.StatusCode,
		"response_body_length", len(respBody),
	)

	if resp.StatusCode != http.StatusOK {
		log.Error("[RPCClient.call] RPC call failed with non-200 status",
			"url", url,
			"method", method,
			"status_code", resp.StatusCode,
			"response_body", string(respBody),
		)
		return nil, fmt.Errorf("rpc call failed with status: %d, body: %s", resp.StatusCode, string(respBody))
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
		log.Error("[RPCClient.call] failed to unmarshal JSON-RPC response",
			"url", url,
			"method", method,
			"response_body", string(respBody),
			"error", err,
		)
		return nil, fmt.Errorf("failed to unmarshal JSON-RPC response: %w", err)
	}

	if rpcResp.Error != nil {
		log.Error("[RPCClient.call] JSON-RPC error",
			"url", url,
			"method", method,
			"error_code", rpcResp.Error.Code,
			"error_message", rpcResp.Error.Message,
		)
		return nil, fmt.Errorf("eth rpc error: code=%d msg=%s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	log.Debug("[RPCClient.call] RPC call successful",
		"url", url,
		"method", method,
		"result_length", len(rpcResp.Result),
	)

	return rpcResp.Result, nil
}

func (c *RPCClient) ChainId(ctx context.Context) (uint64, error) {
	log.Debug("[RPCClient.ChainId] calling eth_chainId")

	result, err := c.call(ctx, "eth_chainId", []interface{}{})
	if err != nil {
		log.Error("[RPCClient.ChainId] failed to get chain ID",
			"error", err,
		)
		return 0, err
	}

	var chainIDHex string
	if err := json.Unmarshal(result, &chainIDHex); err != nil {
		log.Error("[RPCClient.ChainId] failed to unmarshal chain ID",
			"result", string(result),
			"error", err,
		)
		return 0, fmt.Errorf("failed to unmarshal chain ID: %w", err)
	}

	chainID, err := parseHexToUint64(chainIDHex)
	if err != nil {
		log.Error("[RPCClient.ChainId] failed to parse chain ID",
			"chain_id_hex", chainIDHex,
			"error", err,
		)
		return 0, err
	}

	log.Debug("[RPCClient.ChainId] got chain ID",
		"chain_id", chainID,
	)
	return chainID, nil
}

func (c *RPCClient) BlockNumber(ctx context.Context) (string, error) {
	log.Debug("[RPCClient.BlockNumber] calling eth_blockNumber")

	result, err := c.call(ctx, "eth_blockNumber", []interface{}{})
	if err != nil {
		log.Error("[RPCClient.BlockNumber] failed to get block number",
			"error", err,
		)
		return "", err
	}

	var blockNumber string
	if err := json.Unmarshal(result, &blockNumber); err != nil {
		log.Error("[RPCClient.BlockNumber] failed to unmarshal block number",
			"result", string(result),
			"error", err,
		)
		return "", fmt.Errorf("failed to unmarshal block number: %w", err)
	}

	log.Debug("[RPCClient.BlockNumber] got block number",
		"block_number", blockNumber,
	)
	return blockNumber, nil
}

func (c *RPCClient) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	log.Debug("[RPCClient.GetBalance] calling eth_getBalance",
		"address", address,
	)

	result, err := c.call(ctx, "eth_getBalance", []interface{}{address, "latest"})
	if err != nil {
		log.Error("[RPCClient.GetBalance] failed to get balance",
			"address", address,
			"error", err,
		)
		return nil, err
	}

	var balanceHex string
	if err := json.Unmarshal(result, &balanceHex); err != nil {
		log.Error("[RPCClient.GetBalance] failed to unmarshal balance",
			"address", address,
			"result", string(result),
			"error", err,
		)
		return nil, fmt.Errorf("failed to unmarshal balance: %w", err)
	}

	balance := new(big.Int)
	balanceHex = strings.TrimPrefix(balanceHex, "0x")
	balance.SetString(balanceHex, 16)

	log.Debug("[RPCClient.GetBalance] got balance",
		"address", address,
		"balance", balance.String(),
	)
	return balance, nil
}

func (c *RPCClient) GetTransactionCount(ctx context.Context, address string) (uint64, error) {
	log.Debug("[RPCClient.GetTransactionCount] calling eth_getTransactionCount",
		"address", address,
	)

	result, err := c.call(ctx, "eth_getTransactionCount", []interface{}{address, "pending"})
	if err != nil {
		log.Error("[RPCClient.GetTransactionCount] failed to get transaction count",
			"address", address,
			"error", err,
		)
		return 0, err
	}

	var nonceHex string
	if err := json.Unmarshal(result, &nonceHex); err != nil {
		log.Error("[RPCClient.GetTransactionCount] failed to unmarshal nonce",
			"address", address,
			"result", string(result),
			"error", err,
		)
		return 0, fmt.Errorf("failed to unmarshal nonce: %w", err)
	}

	nonce, err := parseHexToUint64(nonceHex)
	if err != nil {
		log.Error("[RPCClient.GetTransactionCount] failed to parse nonce",
			"address", address,
			"nonce_hex", nonceHex,
			"error", err,
		)
		return 0, err
	}

	log.Debug("[RPCClient.GetTransactionCount] got nonce",
		"address", address,
		"nonce", nonce,
	)
	return nonce, nil
}

func (c *RPCClient) GasPrice(ctx context.Context) (*big.Int, error) {
	log.Debug("[RPCClient.GasPrice] calling eth_gasPrice")

	result, err := c.call(ctx, "eth_gasPrice", []interface{}{})
	if err != nil {
		log.Error("[RPCClient.GasPrice] failed to get gas price",
			"error", err,
		)
		return nil, err
	}

	var gasPriceHex string
	if err := json.Unmarshal(result, &gasPriceHex); err != nil {
		log.Error("[RPCClient.GasPrice] failed to unmarshal gas price",
			"result", string(result),
			"error", err,
		)
		return nil, fmt.Errorf("failed to unmarshal gas price: %w", err)
	}

	gasPrice := new(big.Int)
	gasPriceHex = strings.TrimPrefix(gasPriceHex, "0x")
	gasPrice.SetString(gasPriceHex, 16)

	log.Debug("[RPCClient.GasPrice] got gas price",
		"gas_price", gasPrice.String(),
	)
	return gasPrice, nil
}

func (c *RPCClient) GetCode(ctx context.Context, address string) (string, error) {
	log.Debug("[RPCClient.GetCode] calling eth_getCode",
		"address", address,
	)

	result, err := c.call(ctx, "eth_getCode", []interface{}{address, "latest"})
	if err != nil {
		log.Error("[RPCClient.GetCode] failed to get code",
			"address", address,
			"error", err,
		)
		return "", err
	}

	var code string
	if err := json.Unmarshal(result, &code); err != nil {
		log.Error("[RPCClient.GetCode] failed to unmarshal code",
			"address", address,
			"result", string(result),
			"error", err,
		)
		return "", fmt.Errorf("failed to unmarshal code: %w", err)
	}

	log.Debug("[RPCClient.GetCode] got code",
		"address", address,
		"code_length", len(code),
	)
	return code, nil
}

type CallArg struct {
	From  string `json:"from,omitempty"`
	To    string `json:"to"`
	Value string `json:"value,omitempty"`
	Data  string `json:"data,omitempty"`
}

func (c *RPCClient) EstimateGas(ctx context.Context, callObj CallArg) (uint64, error) {
	log.Debug("[RPCClient.EstimateGas] calling eth_estimateGas",
		"call_obj", callObj,
	)

	result, err := c.call(ctx, "eth_estimateGas", []interface{}{callObj})
	if err != nil {
		log.Error("[RPCClient.EstimateGas] failed to estimate gas",
			"call_obj", callObj,
			"error", err,
		)
		return 0, err
	}

	var gasHex string
	if err := json.Unmarshal(result, &gasHex); err != nil {
		log.Error("[RPCClient.EstimateGas] failed to unmarshal gas",
			"result", string(result),
			"error", err,
		)
		return 0, fmt.Errorf("failed to unmarshal gas: %w", err)
	}

	gas, err := parseHexToUint64(gasHex)
	if err != nil {
		log.Error("[RPCClient.EstimateGas] failed to parse gas",
			"gas_hex", gasHex,
			"error", err,
		)
		return 0, err
	}

	log.Debug("[RPCClient.EstimateGas] got gas estimate",
		"gas", gas,
	)
	return gas, nil
}

func (c *RPCClient) Call(ctx context.Context, callObj CallArg) (string, error) {
	log.Debug("[RPCClient.Call] calling eth_call",
		"call_obj", callObj,
	)

	result, err := c.call(ctx, "eth_call", []interface{}{callObj, "latest"})
	if err != nil {
		log.Error("[RPCClient.Call] failed to call",
			"call_obj", callObj,
			"error", err,
		)
		return "", err
	}

	var resultStr string
	if err := json.Unmarshal(result, &resultStr); err != nil {
		log.Error("[RPCClient.Call] failed to unmarshal result",
			"result", string(result),
			"error", err,
		)
		return "", fmt.Errorf("failed to unmarshal result: %w", err)
	}

	log.Debug("[RPCClient.Call] got result",
		"result_length", len(resultStr),
	)
	return resultStr, nil
}

func (c *RPCClient) BroadcastRawTransaction(ctx context.Context, rawHex string) (string, error) {
	log.Debug("[RPCClient.BroadcastRawTransaction] calling eth_sendRawTransaction",
		"raw_hex_length", len(rawHex),
	)

	result, err := c.call(ctx, "eth_sendRawTransaction", []interface{}{rawHex})
	if err != nil {
		log.Error("[RPCClient.BroadcastRawTransaction] broadcast failed",
			"error", err,
		)
		return "", err
	}

	var txHash string
	if err := json.Unmarshal(result, &txHash); err != nil {
		log.Error("[RPCClient.BroadcastRawTransaction] failed to unmarshal tx hash",
			"result", string(result),
			"error", err,
		)
		return "", fmt.Errorf("failed to unmarshal tx hash: %w", err)
	}

	log.Info("[RPCClient.BroadcastRawTransaction] broadcast successful",
		"tx_hash", txHash,
	)
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
