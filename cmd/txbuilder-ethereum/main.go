package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	log "github.com/koku-web3/logko"
	"github.com/koku-web3/txbuilder-ethereum/internal/config"
	"github.com/koku-web3/txbuilder-ethereum/internal/service"
)

func main() {
	configPath := flag.String("config", config.DEFAULT_PATH, "path to config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := log.SetupFromTOML(cfg.Path); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup log: %v", err)
		os.Exit(1)
	}

	log.Info("starting txbuilder-ethereum service", "chain_code", cfg.Chain.ChainCode, "chain_id", fmt.Sprintf("%d", cfg.Chain.ChainID), "rpc_url", cfg.Chain.RPCURL, "grpc_host", cfg.GRPC.Host, "grpc_port", cfg.GRPC.Port)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Info("Received shutdown signal, initiating graceful shutdown")
		cancel()
	}()

	grpcSrv := service.NewGRPCServer(cfg.GRPC.Host, cfg.GRPC.Port)
	if err := grpcSrv.Start(ctx); err != nil {
		log.Error("gRPC server error", "error", err)
	}
}
