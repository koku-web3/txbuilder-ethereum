package service

import (
	"context"

	"fmt"
	"net"

	log "github.com/koku-web3/logko"
	txbuilder "github.com/koku-web3/txbuilder-ethereum/grpc"
	"github.com/koku-web3/txbuilder-ethereum/internal/ethereum"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	BaseCoinFixGas = 21000  // eth 转账恒定基础手续费
	DefaultGas     = 100000 // 100,000
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
