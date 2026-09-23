package service

import (
	"context"
	"encoding/hex"
	std_errors "errors"
	"fmt"
	"math/big"
	"net"
	"strings"

	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/ethereum"
	pkg_errors "github.com/pkg/errors"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const (
	BaseCoinFixGas = 21000
)

// TxBuilderService Ethereum 交易构建服务
// 负责验证地址、检查余额、构建交易签名数据、广播交易
type TxBuilderService struct {
	txbuilder.UnimplementedTxBuilderServer
	rpc *ethereum.RPCClient // Ethereum JSON-RPC 客户端
}

// GRPCServer gRPC 服务器封装
type GRPCServer struct {
	grpcSrv *ggrpc.Server
	host    string
	port    int
}

// NewTxBuilderService 创建交易构建服务实例
func NewTxBuilderService() *TxBuilderService {
	return &TxBuilderService{
		rpc: ethereum.NewRPCClient(),
	}
}

// NewGRPCServer 创建 gRPC 服务器实例
func NewGRPCServer(host string, port int) *GRPCServer {
	return &GRPCServer{
		host: host,
		port: port,
	}
}

// Start 启动 gRPC 服务器，监听指定地址并处理请求
// 使用 context 控制生命周期：ctx.Done() 时优雅关闭服务器
func (s *GRPCServer) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.grpcSrv = ggrpc.NewServer()
	txbuilder.RegisterTxBuilderServer(s.grpcSrv, NewTxBuilderService())
	reflection.Register(s.grpcSrv)

	log.Info("gRPC server starting", "address", addr)

	go func() {
		<-ctx.Done()
		log.Info("gRPC server shutting down")
		s.grpcSrv.GracefulStop()
	}()

	return s.grpcSrv.Serve(lis)
}

// verifyAddress 验证地址的通用逻辑
// validateFn 是具体的验证函数（如 ValidateAddress 或 ValidateContractAddress）
func (s *TxBuilderService) verifyAddress(methodName, traceId, address string, validateFn func(string) bool) (bool, error) {
	if traceId == "" {
		log.Warn("["+methodName+"] Missing trace_id", "trace_id", traceId, "address", address)
		return false, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if address == "" {
		log.Warn("["+methodName+"] Missing address", "trace_id", traceId, "address", address)
		return false, status.Error(codes.InvalidArgument, "address is required")
	}

	isValid := validateFn(address)
	return isValid, nil
}

// VerifyAddress 验证普通以太坊地址格式（EOA 地址）
// 支持 EIP-55 校验和格式验证：
// - 如果地址是 EIP-55 格式（混合大小写），则验证校验和是否正确
// - 如果地址是纯小写或纯大写，则只验证基本格式
func (s *TxBuilderService) VerifyAddress(ctx context.Context, req *txbuilder.VerifyAddressRequest) (*txbuilder.VerifyAddressResponse, error) {
	isValid, err := s.verifyAddress("VerifyAddress", req.TraceId, req.Address, ethereum.ValidateAddress)
	if err != nil {
		return nil, err
	}
	if !isValid {
		return &txbuilder.VerifyAddressResponse{IsValid: false}, nil
	}

	// 如果地址是 EIP-55 格式，验证校验和
	if ethereum.IsEIP55Format(req.Address) {
		isValid = ethereum.VerifyChecksum(req.Address)
	}

	return &txbuilder.VerifyAddressResponse{IsValid: isValid}, nil
}

// VerifyContractAddress 验证智能合约地址格式
// 智能合约地址与普通地址格式相同（0x 开头 + 40 位十六进制），但需要通过 eth_call 验证合约是否存在
func (s *TxBuilderService) VerifyContractAddress(ctx context.Context, req *txbuilder.VerifyContractAddressRequest) (*txbuilder.VerifyContractAddressResponse, error) {
	isValid, err := s.verifyAddress("VerifyContractAddress", req.TraceId, req.Address, ethereum.ValidateContractAddress)
	if err != nil {
		return nil, err
	}
	return &txbuilder.VerifyContractAddressResponse{IsValid: isValid}, nil
}

// ConvertAddress 将 PEM-encoded PKIX 格式公钥转换为 Ethereum 地址
func (s *TxBuilderService) ConvertAddress(ctx context.Context, req *txbuilder.ConvertAddressRequest) (*txbuilder.ConvertAddressResponse, error) {
	log.Info("[ConvertAddress] Request received", "trace_id", req.TraceId, "keys_count", len(req.Keys))

	if err := s.validateConvertAddress(req); err != nil {
		log.Warn("[ConvertAddress] Validation failed", "trace_id", req.TraceId, "error", err.Error())
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	results := make([]*txbuilder.PublicKeysResponse, 0, len(req.Keys))
	for _, key := range req.Keys {
		addr, err := ethereum.PublicKeyPEMToAddress(key.PkixPubkeyPem)
		if err != nil {
			log.Warn("[ConvertAddress] Failed to convert public key", "trace_id", req.TraceId, "account_index", key.AccountIndex, "error", err.Error())
			return nil, status.Errorf(codes.InvalidArgument, "failed to convert public key at account_index %d: %s", key.AccountIndex, err.Error())
		}
		results = append(results, &txbuilder.PublicKeysResponse{
			AccountIndex: key.AccountIndex,
			Address:      addr,
		})
	}

	log.Info("[ConvertAddress] Response returned", "trace_id", req.TraceId, "success_count", len(results))

	return &txbuilder.ConvertAddressResponse{Keys: results}, nil
}

func (*TxBuilderService) validateConvertAddress(req *txbuilder.ConvertAddressRequest) error {
	if req.TraceId == "" || len(req.TraceId) > 36 {
		return std_errors.New("trace_id is required and must be 1-36 characters")
	}
	if len(req.Keys) == 0 {
		return std_errors.New("keys is required and must not be empty")
	}
	if len(req.Keys) > 100 {
		return std_errors.New("keys length exceeds maximum of 100")
	}
	for _, key := range req.Keys {
		if strings.TrimSpace(key.PkixPubkeyPem) == "" {
			return std_errors.New("pkix_pubkey_pem is required and must not be empty")
		}
		if len(key.PkixPubkeyPem) > 4096 {
			return std_errors.New("pkix_pubkey_pem exceeds maximum length of 4096 characters")
		}
	}
	return nil
}

// CheckSufficientBalance 检查地址余额是否足够
// - is_basic_coin=true: 检查 ETH 余额是否 >= 转账金额
// - is_basic_coin=false: 先检查 ETH 余额是否足够支付 Gas，再检查 Token 余额是否 >= 转账金额
//
// 注意：根据 proto 定义，amount 参数已经是链上最小单位（如以太坊是 wei），
//
//	因此 balanceOf 返回的链上单位可直接与 amount 比较，无需 decimals 转换
func (s *TxBuilderService) CheckSufficientBalance(ctx context.Context, req *txbuilder.CheckSufficientBalanceRequest) (*txbuilder.CheckSufficientBalanceResponse, error) {
	if err := s.validateCheckSufficientBalance(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	isContract := req.Contract != ""

	amountInt, err := parseAmount(req.Amount)
	if err != nil {
		log.Warn("[CheckSufficientBalance] Invalid amount format", "trace_id", req.TraceId, "amount", req.Amount, "error", err)
		return nil, status.Error(codes.InvalidArgument, "invalid amount format")
	}

	balance, err := s.getBasicCoinBalance(ctx, req.FromAddress)
	if err != nil {
		log.Error("[CheckSufficientBalance] Failed to get balance", "trace_id", req.TraceId, "from_address", req.FromAddress, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get balance: %v", err)
	}

	var calldata string
	if isContract {
		calldata = hex.EncodeToString(buildERC20TransferData(req.FromAddress, amountInt))
	}

	fee, err := s.getEip1559TxFee(ctx, isContract, req.FromAddress, req.Contract, calldata)
	if err != nil {
		log.Error("[CheckSufficientBalance] Failed to get fee", "trace_id", req.TraceId, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get fee: %v", err)
	}

	if !isContract {
		// 原生币转账：余额需要 >= 金额 + 手续费
		isCoinSufficient := balance.Cmp(new(big.Int).Add(amountInt, fee)) >= 0
		log.Info("[CheckSufficientBalance] Balance check result", "trace_id", req.TraceId, "balance", balance.String(), "required", new(big.Int).Add(amountInt, fee).String(), "is_coin_sufficient", isCoinSufficient)
		return &txbuilder.CheckSufficientBalanceResponse{IsCoinSufficient: isCoinSufficient}, nil
	}

	if !ethereum.ValidateContractAddress(req.Contract) {
		return nil, status.Error(codes.InvalidArgument, "invalid contract format")
	}

	// Token 转账：只需检查 ETH 余额是否足够支付 Gas
	isCoinSufficient := balance.Cmp(fee) >= 0

	tokenBalance, err := s.getTokenBalance(ctx, req.FromAddress, req.Contract)
	if err != nil {
		log.Error("[CheckSufficientBalance] Failed to get token balance", "trace_id", req.TraceId, "from_address", req.FromAddress, "contract", req.Contract, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get token balance: %v", err)
	}

	// amount 已是链上最小单位，与 balanceOf 返回值直接比较
	isTokenSufficient := tokenBalance.Cmp(amountInt) >= 0

	log.Info("[CheckSufficientBalance] Token balance check result", "trace_id", req.TraceId, "token_balance", tokenBalance.String(), "required_amount", amountInt.String(), "is_coin_sufficient", isCoinSufficient, "is_token_sufficient", isTokenSufficient)

	return &txbuilder.CheckSufficientBalanceResponse{IsCoinSufficient: isCoinSufficient, IsTokenSufficient: isTokenSufficient}, nil
}

func (*TxBuilderService) validateCheckSufficientBalance(req *txbuilder.CheckSufficientBalanceRequest) error {
	if req.TraceId == "" {
		log.Warn("[CheckSufficientBalance] Validation failed", "trace_id", req.TraceId, "field", "trace_id", "reason", "required")
		return status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if req.ChainCode == "" || len(req.ChainCode) > 36 {
		log.Warn("[CheckSufficientBalance] Validation failed", "trace_id", req.TraceId, "field", "chain_code", "reason", "required and must be 1-36 characters")
		return status.Error(codes.InvalidArgument, "chain_code is required and must be 1-36 characters")
	}
	if req.Coin == "" || len(req.Coin) > 36 {
		log.Warn("[CheckSufficientBalance] Validation failed", "trace_id", req.TraceId, "field", "coin", "reason", "required and must be 1-36 characters")
		return status.Error(codes.InvalidArgument, "coin_id is required and must be 1-36 characters")
	}
	if req.FromAddress == "" || len(req.FromAddress) > 256 {
		log.Warn("[CheckSufficientBalance] Validation failed", "trace_id", req.TraceId, "field", "from_address", "reason", "required and must be 1-256 characters")
		return status.Error(codes.InvalidArgument, "from_address is required and must be 1-256 characters")
	}
	if req.Amount == "" || len(req.Amount) > 64 {
		log.Warn("[CheckSufficientBalance] Validation failed", "trace_id", req.TraceId, "field", "amount", "reason", "required and must be 1-64 characters")
		return status.Error(codes.InvalidArgument, "amount is required and must be 1-64 characters")
	}
	return nil
}

func (s *TxBuilderService) validateBuildSignRawData(req *txbuilder.BuildSignRawDataRequest) error {
	if req.TraceId == "" || len(req.TraceId) > 36 {
		log.Warn("[BuildSignRawData] Validation failed", "trace_id", req.TraceId, "field", "trace_id", "reason", "required and must be 1-36 characters")
		return status.Error(codes.InvalidArgument, "trace_id is required and must be 1-36 characters")
	}
	if req.ChainCode == "" || len(req.ChainCode) > 36 {
		log.Warn("[BuildSignRawData] Validation failed", "trace_id", req.TraceId, "field", "chain_code", "reason", "required and must be 1-36 characters")
		return status.Error(codes.InvalidArgument, "chain_code is required and must be 1-36 characters")
	}
	if req.Coin == "" || len(req.Coin) > 36 {
		log.Warn("[BuildSignRawData] Validation failed", "trace_id", req.TraceId, "field", "coin", "reason", "required and must be 1-36 characters")
		return status.Error(codes.InvalidArgument, "coin_id is required and must be 1-36 characters")
	}
	if req.CoinSymbol == "" || len(req.CoinSymbol) > 36 {
		log.Warn("[BuildSignRawData] Validation failed", "trace_id", req.TraceId, "field", "coin_symbol", "reason", "required and must be 1-36 characters")
		return status.Error(codes.InvalidArgument, "coin_symbol is required and must be 1-36 characters")
	}
	if req.FromAddress == "" || len(req.FromAddress) > 256 {
		log.Warn("[BuildSignRawData] Validation failed", "trace_id", req.TraceId, "field", "from_address", "reason", "required and must be 1-256 characters")
		return status.Error(codes.InvalidArgument, "from_address is required and must be 1-256 characters")
	}
	if req.ToAddress == "" || len(req.ToAddress) > 256 {
		log.Warn("[BuildSignRawData] Validation failed", "trace_id", req.TraceId, "field", "to_address", "reason", "required and must be 1-256 characters")
		return status.Error(codes.InvalidArgument, "to_address is required and must be 1-256 characters")
	}
	if req.Amount == "" || len(req.Amount) > 64 {
		log.Warn("[BuildSignRawData] Validation failed", "trace_id", req.TraceId, "field", "amount", "reason", "required and must be 1-64 characters")
		return status.Error(codes.InvalidArgument, "amount is required and must be 1-64 characters")
	}
	if !ethereum.IsPureNumber(req.Amount) {
		log.Warn("[BuildSignRawData] Validation failed", "trace_id", req.TraceId, "field", "amount", "reason", "must be a pure number")
		return status.Error(codes.InvalidArgument, "amount must be a pure number")
	}
	if !ethereum.ValidateAddress(req.FromAddress) {
		log.Warn("[BuildSignRawData] Validation failed", "trace_id", req.TraceId, "field", "from_address", "reason", "invalid ethereum address")
		return status.Error(codes.InvalidArgument, "invalid from_address")
	}
	if !ethereum.ValidateAddress(req.ToAddress) {
		log.Warn("[BuildSignRawData] Validation failed", "trace_id", req.TraceId, "field", "to_address", "reason", "invalid ethereum address")
		return status.Error(codes.InvalidArgument, "invalid to_address")
	}
	return nil
}

// getBasicCoinBalance 获取 ETH 原生币余额
// 调用 eth_getBalance RPC 方法
func (s *TxBuilderService) getBasicCoinBalance(ctx context.Context, address string) (*big.Int, error) {
	balance, err := s.rpc.GetBalance(ctx, address)
	if err != nil {
		return nil, pkg_errors.Wrap(err, "failed to get balance from node")
	}
	return balance, nil
}

// getTokenBalance 获取 ERC20 Token 余额
// 通过 eth_call 调用合约的 balanceOf 方法（methodID: 0x70a08231）
// 返回的余额是原始数值（未除以精度），需要根据 Token decimals 转换
func (s *TxBuilderService) getTokenBalance(ctx context.Context, owner, contract string) (*big.Int, error) {
	methodID := "0x70a08231"
	paddedOwner := padAddressTo32Bytes(owner)
	data := methodID + paddedOwner

	result, err := s.rpc.Call(ctx, ethereum.CallArg{To: contract, Data: data})
	if err != nil {
		return big.NewInt(0), pkg_errors.Wrap(err, "failed to call contract")
	}

	result = strings.TrimPrefix(result, "0x")
	if len(result) < 64 {
		return big.NewInt(0), nil
	}

	balanceHex := result[len(result)-64:]
	balance := new(big.Int)
	balance.SetString(balanceHex, 16)

	return balance, nil
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
		log.Error("[getEip1559TxFee] Failed to get gas fee cap", "error", err)
		return nil, pkg_errors.Wrap(err, "failed to get gas fee cap")
	}

	if _, err := s.rpc.SuggestGasTipCap(ctx); err != nil {
		log.Warn("[getEip1559TxFee] Failed to get gas tip cap, using fallback 2 gwei", "error", err)
	}

	var gas uint64
	if isContract {
		estimated, err := s.rpc.EstimateGas(ctx, ethereum.CallArg{From: from, To: to, Data: data})
		if err != nil {
			log.Warn("[getEip1559TxFee] Gas estimation failed, using default 100000", "error", err)
			gas = 100000
		} else {
			gas = uint64(float64(estimated) * 1.2)
		}
	} else {
		gas = BaseCoinFixGas
	}

	totalFee := new(big.Int).Mul(gasFeeCap, new(big.Int).SetUint64(gas))
	return totalFee, nil
}

// buildERC20TransferData 构建 ERC20 transfer 调用的 calldata
// 格式：methodID(4字节) + 地址(32字节, 左补零) + 金额(32字节, 左补零)
// transfer(address to, uint256 amount)
func buildERC20TransferData(to string, amount *big.Int) []byte {
	methodID := []byte{0xa9, 0x05, 0x9c, 0xbb}

	paddedAddr := padAddressTo32BytesBytes(to)
	paddedAmount := padAmountTo32Bytes(amount)

	data := make([]byte, 0, 4+32+32)
	data = append(data, methodID...)
	data = append(data, paddedAddr...)
	data = append(data, paddedAmount...)

	return data
}

// padAddressTo32Bytes 将地址补零到 32 字节（Hex 编码字符串）
// Ethereum ABI 编码要求参数固定 32 字节
// 示例：0x1234 -> 0x0000...00001234（左补零至 32 字节）
func padAddressTo32Bytes(address string) string {
	address = strings.TrimPrefix(address, "0x")
	address = strings.ToLower(address)

	addrBytes, err := hex.DecodeString(address)
	if err != nil {
		log.Error("[padAddressTo32Bytes] Failed to decode address", "address", address, "error", err)
		return strings.Repeat("0", 64)
	}

	padded := make([]byte, 32)
	copy(padded[32-len(addrBytes):], addrBytes)
	return hex.EncodeToString(padded)
}

// padAddressTo32BytesBytes 将地址补零到 32 字节（字节数组版本）
// 以太坊地址是 20 字节的 hex 编码（40 个十六进制字符），需要先转换为字节数组再补零
func padAddressTo32BytesBytes(address string) []byte {
	address = strings.TrimPrefix(address, "0x")
	address = strings.ToLower(address)

	addrBytes, err := hex.DecodeString(address)
	if err != nil {
		log.Error("[padAddressTo32BytesBytes] Failed to decode address", "address", address, "error", err)
		return make([]byte, 32)
	}

	padded := make([]byte, 32)
	copy(padded[32-len(addrBytes):], addrBytes)
	return padded
}

// padAmountTo32Bytes 将金额补零到 32 字节
// Ethereum ABI 编码要求 uint256 参数固定 32 字节
// 示例：大数 0x1234567890 -> 0x0000...001234567890（左补零至 32 字节）
func padAmountTo32Bytes(amount *big.Int) []byte {
	amountBytes := amount.Bytes()
	padded := make([]byte, 32)
	copy(padded[32-len(amountBytes):], amountBytes)
	return padded
}

// parseAmount 将字符串金额解析为 *big.Int（单位：wei）
// 金额格式为十进制字符串（如 "1000000000000000000" 表示 1 ETH）
func parseAmount(amount string) (*big.Int, error) {
	amountInt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, std_errors.New("invalid amount format")
	}
	return amountInt, nil
}
