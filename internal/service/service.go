package service

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/koku-web3/logko"
	"github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/ethereum"
	"github.com/pkg/errors"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type TxBuilderService struct {
	grpc.UnimplementedTxBuilderServer
	rpc *ethereum.RPCClient
}

type GRPCServer struct {
	grpcSrv *ggrpc.Server
	host    string
	port    int
}

func NewTxBuilderService() *TxBuilderService {
	return &TxBuilderService{
		rpc: ethereum.NewRPCClient(),
	}
}

func NewGRPCServer(host string, port int) *GRPCServer {
	return &GRPCServer{
		host: host,
		port: port,
	}
}

func (s *GRPCServer) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.grpcSrv = ggrpc.NewServer()
	grpc.RegisterTxBuilderServer(s.grpcSrv, NewTxBuilderService())
	reflection.Register(s.grpcSrv)

	log.Info("Starting gRPC server", "address", addr)

	go func() {
		<-ctx.Done()
		log.Info("Shutting down gRPC server")
		s.grpcSrv.GracefulStop()
	}()

	return s.grpcSrv.Serve(lis)
}

func (s *TxBuilderService) verifyAddress(ctx context.Context, methodName, traceId, address string, validateFn func(string) bool) (bool, error) {
	log.Info("["+methodName+"] received request",
		"trace_id", traceId,
		"address", address,
	)

	if traceId == "" {
		log.Warn("["+methodName+"] missing trace_id",
			"trace_id", traceId,
			"address", address,
		)
		return false, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if address == "" {
		log.Warn("["+methodName+"] missing address",
			"trace_id", traceId,
			"address", address,
		)
		return false, status.Error(codes.InvalidArgument, "address is required")
	}

	isValid := validateFn(address)

	log.Info("["+methodName+"] returning response",
		"trace_id", traceId,
		"address", address,
		"is_valid", isValid,
	)

	return isValid, nil
}

func (s *TxBuilderService) VerifyAddress(ctx context.Context, req *grpc.VerifyAddressRequest) (*grpc.VerifyAddressResponse, error) {
	isValid, err := s.verifyAddress(ctx, "VerifyAddress", req.TraceId, req.Address, ethereum.ValidateAddress)
	if err != nil {
		return nil, err
	}
	return &grpc.VerifyAddressResponse{IsValid: isValid}, nil
}

func (s *TxBuilderService) VerifyContractAddress(ctx context.Context, req *grpc.VerifyContractAddressRequest) (*grpc.VerifyContractAddressResponse, error) {
	isValid, err := s.verifyAddress(ctx, "VerifyContractAddress", req.TraceId, req.Address, ethereum.ValidateContractAddress)
	if err != nil {
		return nil, err
	}
	return &grpc.VerifyContractAddressResponse{IsValid: isValid}, nil
}

func (s *TxBuilderService) CheckSufficientBalance(ctx context.Context, req *grpc.CheckSufficientBalanceRequest) (*grpc.CheckSufficientBalanceResponse, error) {
	log.Info("[CheckSufficientBalance] received request",
		"trace_id", req.TraceId,
		"chain_code", req.ChainCode,
		"coin_id", req.CoinId,
		"is_basic_coin", req.IsBasicCoin,
		"from_address", req.FromAddress,
		"amount", req.Amount,
		"contract", req.Contract,
	)

	if req.TraceId == "" {
		log.Warn("[CheckSufficientBalance] missing trace_id",
			"trace_id", req.TraceId,
		)
		return nil, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if req.ChainCode == "" || len(req.ChainCode) > 36 {
		log.Warn("[CheckSufficientBalance] invalid chain_code",
			"trace_id", req.TraceId,
			"chain_code", req.ChainCode,
			"chain_code_length", len(req.ChainCode),
		)
		return nil, status.Error(codes.InvalidArgument, "chain_code is required and must be 1-36 characters")
	}
	if req.CoinId == "" || len(req.CoinId) > 36 {
		log.Warn("[CheckSufficientBalance] invalid coin_id",
			"trace_id", req.TraceId,
			"coin_id", req.CoinId,
			"coin_id_length", len(req.CoinId),
		)
		return nil, status.Error(codes.InvalidArgument, "coin_id is required and must be 1-36 characters")
	}
	if req.FromAddress == "" || len(req.FromAddress) > 256 {
		log.Warn("[CheckSufficientBalance] invalid from_address",
			"trace_id", req.TraceId,
			"from_address", req.FromAddress,
			"from_address_length", len(req.FromAddress),
		)
		return nil, status.Error(codes.InvalidArgument, "from_address is required and must be 1-256 characters")
	}
	if req.Amount == "" || len(req.Amount) > 64 {
		log.Warn("[CheckSufficientBalance] invalid amount",
			"trace_id", req.TraceId,
			"amount", req.Amount,
			"amount_length", len(req.Amount),
		)
		return nil, status.Error(codes.InvalidArgument, "amount is required and must be 1-64 characters")
	}
	if !req.IsBasicCoin && (req.Contract == "" || len(req.Contract) > 256) {
		log.Warn("[CheckSufficientBalance] missing contract for token",
			"trace_id", req.TraceId,
			"is_basic_coin", req.IsBasicCoin,
			"contract", req.Contract,
		)
		return nil, status.Error(codes.InvalidArgument, "contract is required when is_basic_coin is false")
	}

	log.Info("[CheckSufficientBalance] validation passed, checking balance",
		"trace_id", req.TraceId,
		"is_basic_coin", req.IsBasicCoin,
	)

	if req.IsBasicCoin {
		log.Info("[CheckSufficientBalance] checking basic coin (ETH) balance",
			"trace_id", req.TraceId,
			"from_address", req.FromAddress,
		)

		balance, err := s.getBasicCoinBalance(ctx, req.FromAddress)
		if err != nil {
			log.Error("[CheckSufficientBalance] failed to get basic coin balance",
				"trace_id", req.TraceId,
				"from_address", req.FromAddress,
				"error", err,
			)
			return nil, status.Errorf(codes.Internal, "failed to get balance: %v", err)
		}

		log.Info("[CheckSufficientBalance] got basic coin balance",
			"trace_id", req.TraceId,
			"from_address", req.FromAddress,
			"balance", balance.String(),
		)

		amountInt, err := parseAmount(req.Amount)
		if err != nil {
			log.Warn("[CheckSufficientBalance] invalid amount format",
				"trace_id", req.TraceId,
				"amount", req.Amount,
				"error", err,
			)
			return nil, status.Error(codes.InvalidArgument, "invalid amount format")
		}

		isSufficient := balance.Cmp(amountInt) >= 0

		log.Info("[CheckSufficientBalance] basic coin balance check result",
			"trace_id", req.TraceId,
			"balance", balance.String(),
			"required_amount", amountInt.String(),
			"is_sufficient", isSufficient,
		)

		return &grpc.CheckSufficientBalanceResponse{IsSufficient: isSufficient}, nil
	}

	log.Info("[CheckSufficientBalance] checking token balance",
		"trace_id", req.TraceId,
		"from_address", req.FromAddress,
		"contract", req.Contract,
	)

	baseFee, err := s.getBaseFee()
	if err != nil {
		log.Error("[CheckSufficientBalance] failed to get base fee",
			"trace_id", req.TraceId,
			"error", err,
		)
		return nil, status.Errorf(codes.Internal, "failed to get base fee: %v", err)
	}

	log.Info("[CheckSufficientBalance] got base fee",
		"trace_id", req.TraceId,
		"base_fee", baseFee.String(),
	)

	balance, err := s.getBasicCoinBalance(ctx, req.FromAddress)
	if err != nil {
		log.Error("[CheckSufficientBalance] failed to get ETH balance for fee",
			"trace_id", req.TraceId,
			"from_address", req.FromAddress,
			"error", err,
		)
		return nil, status.Errorf(codes.Internal, "failed to get balance: %v", err)
	}

	log.Info("[CheckSufficientBalance] got ETH balance for fee check",
		"trace_id", req.TraceId,
		"balance", balance.String(),
		"base_fee", baseFee.String(),
	)

	if balance.Cmp(baseFee) < 0 {
		log.Warn("[CheckSufficientBalance] insufficient ETH for fee",
			"trace_id", req.TraceId,
			"balance", balance.String(),
			"base_fee", baseFee.String(),
		)
		return &grpc.CheckSufficientBalanceResponse{IsSufficient: false}, nil
	}

	tokenBalance, err := s.getTokenBalance(ctx, req.FromAddress, req.Contract)
	if err != nil {
		log.Error("[CheckSufficientBalance] failed to get token balance",
			"trace_id", req.TraceId,
			"from_address", req.FromAddress,
			"contract", req.Contract,
			"error", err,
		)
		return nil, status.Errorf(codes.Internal, "failed to get token balance: %v", err)
	}

	log.Info("[CheckSufficientBalance] got token balance",
		"trace_id", req.TraceId,
		"from_address", req.FromAddress,
		"contract", req.Contract,
		"token_balance", tokenBalance.String(),
	)

	amountInt, err := parseAmount(req.Amount)
	if err != nil {
		log.Warn("[CheckSufficientBalance] invalid amount format",
			"trace_id", req.TraceId,
			"amount", req.Amount,
			"error", err,
		)
		return nil, status.Error(codes.InvalidArgument, "invalid amount format")
	}

	isSufficient := tokenBalance.Cmp(amountInt) >= 0

	log.Info("[CheckSufficientBalance] token balance check result",
		"trace_id", req.TraceId,
		"token_balance", tokenBalance.String(),
		"required_amount", amountInt.String(),
		"is_sufficient", isSufficient,
	)

	return &grpc.CheckSufficientBalanceResponse{IsSufficient: isSufficient}, nil
}

func (s *TxBuilderService) BuildSignRawData(ctx context.Context, req *grpc.BuildSignRawDataRequest) (*grpc.BuildSignRawDataResponse, error) {
	log.Info("[BuildSignRawData] received request",
		"biz_id", req.BizId,
		"chain_code", req.ChainCode,
		"coin_id", req.CoinId,
		"coin_symbol", req.CoinSymbol,
		"is_basic_coin", req.IsBasicCoin,
		"from_address", req.FromAddress,
		"to_address", req.ToAddress,
		"amount", req.Amount,
		"contract", req.Contract,
	)

	if req.BizId == "" || len(req.BizId) > 36 {
		log.Warn("[BuildSignRawData] invalid biz_id",
			"biz_id", req.BizId,
			"biz_id_length", len(req.BizId),
		)
		return nil, status.Error(codes.InvalidArgument, "biz_id is required and must be 1-36 characters")
	}
	if req.ChainCode == "" || len(req.ChainCode) > 36 {
		log.Warn("[BuildSignRawData] invalid chain_code",
			"biz_id", req.BizId,
			"chain_code", req.ChainCode,
			"chain_code_length", len(req.ChainCode),
		)
		return nil, status.Error(codes.InvalidArgument, "chain_code is required and must be 1-36 characters")
	}
	if req.CoinId == "" || len(req.CoinId) > 36 {
		log.Warn("[BuildSignRawData] invalid coin_id",
			"biz_id", req.BizId,
			"coin_id", req.CoinId,
			"coin_id_length", len(req.CoinId),
		)
		return nil, status.Error(codes.InvalidArgument, "coin_id is required and must be 1-36 characters")
	}
	if req.CoinSymbol == "" || len(req.CoinSymbol) > 36 {
		log.Warn("[BuildSignRawData] invalid coin_symbol",
			"biz_id", req.BizId,
			"coin_symbol", req.CoinSymbol,
			"coin_symbol_length", len(req.CoinSymbol),
		)
		return nil, status.Error(codes.InvalidArgument, "coin_symbol is required and must be 1-36 characters")
	}
	if req.FromAddress == "" || len(req.FromAddress) > 256 {
		log.Warn("[BuildSignRawData] invalid from_address",
			"biz_id", req.BizId,
			"from_address", req.FromAddress,
			"from_address_length", len(req.FromAddress),
		)
		return nil, status.Error(codes.InvalidArgument, "from_address is required and must be 1-256 characters")
	}
	if req.ToAddress == "" || len(req.ToAddress) > 256 {
		log.Warn("[BuildSignRawData] invalid to_address",
			"biz_id", req.BizId,
			"to_address", req.ToAddress,
			"to_address_length", len(req.ToAddress),
		)
		return nil, status.Error(codes.InvalidArgument, "to_address is required and must be 1-256 characters")
	}
	if req.Amount == "" || len(req.Amount) > 64 {
		log.Warn("[BuildSignRawData] invalid amount",
			"biz_id", req.BizId,
			"amount", req.Amount,
			"amount_length", len(req.Amount),
		)
		return nil, status.Error(codes.InvalidArgument, "amount is required and must be 1-64 characters")
	}
	if !req.IsBasicCoin && (req.Contract == "" || len(req.Contract) > 256) {
		log.Warn("[BuildSignRawData] missing contract for token",
			"biz_id", req.BizId,
			"is_basic_coin", req.IsBasicCoin,
			"contract", req.Contract,
		)
		return nil, status.Error(codes.InvalidArgument, "contract is required when is_basic_coin is false")
	}

	if !ethereum.IsPureNumber(req.Amount) {
		log.Warn("[BuildSignRawData] amount is not pure number",
			"biz_id", req.BizId,
			"amount", req.Amount,
		)
		return nil, status.Error(codes.InvalidArgument, "amount must be a pure number")
	}

	if !ethereum.ValidateAddress(req.FromAddress) {
		log.Warn("[BuildSignRawData] invalid from_address",
			"biz_id", req.BizId,
			"from_address", req.FromAddress,
		)
		return nil, status.Error(codes.InvalidArgument, "invalid from_address")
	}
	if !ethereum.ValidateAddress(req.ToAddress) {
		log.Warn("[BuildSignRawData] invalid to_address",
			"biz_id", req.BizId,
			"to_address", req.ToAddress,
		)
		return nil, status.Error(codes.InvalidArgument, "invalid to_address")
	}

	log.Info("[BuildSignRawData] validation passed, building transaction",
		"biz_id", req.BizId,
		"is_basic_coin", req.IsBasicCoin,
	)

	var msg, rawData string
	var err error

	if req.IsBasicCoin {
		log.Info("[BuildSignRawData] building basic coin (ETH) transaction",
			"biz_id", req.BizId,
			"from_address", req.FromAddress,
			"to_address", req.ToAddress,
			"amount", req.Amount,
		)

		msg, rawData, err = s.buildBasicCoinTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount)
	} else {
		log.Info("[BuildSignRawData] building token transaction",
			"biz_id", req.BizId,
			"from_address", req.FromAddress,
			"to_address", req.ToAddress,
			"amount", req.Amount,
			"contract", req.Contract,
		)

		msg, rawData, err = s.buildTokenTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount, req.Contract)
	}

	if err != nil {
		log.Error("[BuildSignRawData] failed to build transaction",
			"biz_id", req.BizId,
			"is_basic_coin", req.IsBasicCoin,
			"error", err,
		)
		return nil, status.Errorf(codes.Internal, "failed to build transaction: %v", err)
	}

	log.Info("[BuildSignRawData] transaction built successfully",
		"biz_id", req.BizId,
		"msg_length", len(msg),
		"raw_data_length", len(rawData),
	)

	resp := &grpc.BuildSignRawDataResponse{
		Msg:     msg,
		RawData: rawData,
	}

	log.Info("[BuildSignRawData] returning response",
		"biz_id", req.BizId,
		"msg", msg,
		"raw_data_length", len(rawData),
	)

	return resp, nil
}

func (s *TxBuilderService) TxBroadcast(ctx context.Context, req *grpc.TxBroadcastRequest) (*grpc.TxBroadcastResponse, error) {
	log.Info("[TxBroadcast] received request",
		"trace_id", req.TraceId,
		"raw_data_length", len(req.RawData),
		"signature_length", len(req.Signature),
	)

	if req.TraceId == "" {
		log.Warn("[TxBroadcast] missing trace_id",
			"trace_id", req.TraceId,
		)
		return nil, status.Error(codes.InvalidArgument, "trace_id is required")
	}
	if req.RawData == "" {
		log.Warn("[TxBroadcast] missing raw_data",
			"trace_id", req.TraceId,
		)
		return nil, status.Error(codes.InvalidArgument, "raw_data is required")
	}
	if len(req.RawData) < 1 || len(req.RawData) > 4096 {
		log.Warn("[TxBroadcast] invalid raw_data length",
			"trace_id", req.TraceId,
			"raw_data_length", len(req.RawData),
		)
		return nil, status.Error(codes.InvalidArgument, "raw_data must be 1-4096 characters")
	}

	log.Info("[TxBroadcast] validation passed, broadcasting transaction",
		"trace_id", req.TraceId,
	)

	_, err := s.rpc.BroadcastRawTransaction(ctx, req.RawData)
	if err != nil {
		log.Error("[TxBroadcast] failed to broadcast transaction",
			"trace_id", req.TraceId,
			"error", err,
		)
		return nil, status.Errorf(codes.Internal, "failed to broadcast transaction: %v", err)
	}

	log.Info("[TxBroadcast] returning response",
		"trace_id", req.TraceId,
		"success", true,
	)

	return &grpc.TxBroadcastResponse{Success: true}, nil
}

func (s *TxBuilderService) getBasicCoinBalance(ctx context.Context, address string) (*big.Int, error) {
	log.Debug("[getBasicCoinBalance] getting ETH balance",
		"address", address,
	)

	balance, err := s.rpc.GetBalance(ctx, address)
	if err != nil {
		log.Error("[getBasicCoinBalance] failed to get balance from node",
			"address", address,
			"error", err,
		)
		return nil, errors.Wrap(err, "failed to get balance from node")
	}

	log.Debug("[getBasicCoinBalance] got balance from node",
		"address", address,
		"balance", balance.String(),
	)

	return balance, nil
}

func (s *TxBuilderService) getTokenBalance(ctx context.Context, owner, contract string) (*big.Int, error) {
	log.Debug("[getTokenBalance] getting token balance",
		"owner", owner,
		"contract", contract,
	)

	methodID := "0x70a08231"

	paddedOwner := padAddressTo32Bytes(owner)
	data := methodID + paddedOwner

	log.Debug("[getTokenBalance] calling eth_call",
		"to", contract,
		"data", data,
	)

	result, err := s.rpc.Call(ctx, ethereum.CallArg{
		To:   contract,
		Data: data,
	})
	if err != nil {
		log.Error("[getTokenBalance] failed to call contract",
			"owner", owner,
			"contract", contract,
			"error", err,
		)
		return big.NewInt(0), errors.Wrap(err, "failed to call contract")
	}

	result = strings.TrimPrefix(result, "0x")
	if len(result) < 64 {
		log.Debug("[getTokenBalance] no balance or zero balance",
			"owner", owner,
			"contract", contract,
		)
		return big.NewInt(0), nil
	}

	balanceHex := result[len(result)-64:]
	balance := new(big.Int)
	balance.SetString(balanceHex, 16)

	log.Debug("[getTokenBalance] got token balance",
		"owner", owner,
		"contract", contract,
		"balance", balance.String(),
	)

	return balance, nil
}

func (s *TxBuilderService) getBaseFee() (*big.Int, error) {
	log.Debug("[getBaseFee] returning base fee")
	return big.NewInt(1000000), nil
}

func (s *TxBuilderService) buildBasicCoinTransaction(ctx context.Context, from, to, amount string) (string, string, error) {
	log.Debug("[buildBasicCoinTransaction] building ETH transfer",
		"from", from,
		"to", to,
		"amount", amount,
	)

	amountInt, err := parseAmount(amount)
	if err != nil {
		return "", "", errors.Wrap(err, "invalid amount")
	}

	nonce, err := s.rpc.GetTransactionCount(ctx, from)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get nonce")
	}

	gasPrice, err := s.rpc.GasPrice(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get gas price")
	}

	toAddress := common.HexToAddress(to)

	tx := types.NewTransaction(
		nonce,
		toAddress,
		amountInt,
		21000,
		gasPrice,
		nil,
	)

	signer := types.NewEIP155Signer(big.NewInt(int64(s.rpc.ChainID)))
	hash := signer.Hash(tx)

	msg := hex.EncodeToString(hash.Bytes())

	var rawData []byte
	if tx.Type() == types.LegacyTxType {
		buf := new(bytes.Buffer)
		if err := tx.EncodeRLP(buf); err != nil {
			return "", "", errors.Wrap(err, "failed to encode transaction")
		}
		rawData = buf.Bytes()
	}

	log.Debug("[buildBasicCoinTransaction] transaction built",
		"from", from,
		"to", to,
		"nonce", nonce,
		"gas_price", gasPrice.String(),
		"chain_id", s.rpc.ChainID,
		"msg", msg,
	)

	return msg, hex.EncodeToString(rawData), nil
}

func (s *TxBuilderService) buildTokenTransaction(ctx context.Context, from, to, amount, contract string) (string, string, error) {
	log.Debug("[buildTokenTransaction] building ERC20 transfer",
		"from", from,
		"to", to,
		"amount", amount,
		"contract", contract,
	)

	amountInt, err := parseAmount(amount)
	if err != nil {
		return "", "", errors.Wrap(err, "invalid amount")
	}

	nonce, err := s.rpc.GetTransactionCount(ctx, from)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get nonce")
	}

	gasPrice, err := s.rpc.GasPrice(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get gas price")
	}

	data := buildERC20TransferData(to, amountInt)

	gas, err := s.rpc.EstimateGas(ctx, ethereum.CallArg{
		From: from,
		To:   contract,
		Data: string(data),
	})
	if err != nil {
		log.Warn("[buildTokenTransaction] gas estimation failed, using default",
			"error", err,
		)
		gas = 100000
	} else {
		gas = uint64(float64(gas) * 1.2)
	}

	toAddress := common.HexToAddress(contract)

	tx := types.NewTransaction(
		nonce,
		toAddress,
		big.NewInt(0),
		gas,
		gasPrice,
		data,
	)

	signer := types.NewEIP155Signer(big.NewInt(int64(s.rpc.ChainID)))
	hash := signer.Hash(tx)

	msg := hex.EncodeToString(hash.Bytes())

	var rawData []byte
	if tx.Type() == types.LegacyTxType {
		buf := new(bytes.Buffer)
		if err := tx.EncodeRLP(buf); err != nil {
			return "", "", errors.Wrap(err, "failed to encode transaction")
		}
		rawData = buf.Bytes()
	}

	log.Debug("[buildTokenTransaction] ERC20 transaction built",
		"from", from,
		"to", to,
		"contract", contract,
		"nonce", nonce,
		"gas", gas,
		"chain_id", s.rpc.ChainID,
		"msg", msg,
	)

	return msg, hex.EncodeToString(rawData), nil
}

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

func padAddressTo32Bytes(address string) string {
	address = strings.TrimPrefix(address, "0x")
	address = strings.ToLower(address)

	padded := make([]byte, 32)
	copy(padded[32-len(address):], address)
	return hex.EncodeToString(padded)
}

func padAddressTo32BytesBytes(address string) []byte {
	address = strings.TrimPrefix(address, "0x")
	address = strings.ToLower(address)

	padded := make([]byte, 32)
	copy(padded[32-len(address):], address)
	return padded
}

func padAmountTo32Bytes(amount *big.Int) []byte {
	amountBytes := amount.Bytes()
	padded := make([]byte, 32)
	copy(padded[32-len(amountBytes):], amountBytes)
	return padded
}

func parseAmount(amount string) (*big.Int, error) {
	amountInt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, errors.New("invalid amount format")
	}
	return amountInt, nil
}
