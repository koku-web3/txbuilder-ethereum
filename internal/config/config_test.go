package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	configContent := `
[chain]
chain_code = "ethereum"
rpc_url = "https://rpc.sepolia.org"
chain_id = 11155111

[grpc]
host = "127.0.0.1"
port = 50052

[log]
rotation = true
file_path = "/tmp/test.log"
max_size_mb = 100
max_backups = 10
max_age = 30
compress = true
format = "terminal"
verbosity = 3
vmodule = ""
`

	tmpFile, err := os.CreateTemp("", "config_test_*.toml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	_, err = LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	cfg := GetConfig()
	if cfg == nil {
		t.Fatal("GetConfig() returned nil")
	}

	if cfg.Chain.ChainCode != "ethereum" {
		t.Errorf("ChainCode = %v, want %v", cfg.Chain.ChainCode, "ethereum")
	}
	if cfg.Chain.RPCURL != "https://rpc.sepolia.org" {
		t.Errorf("RPCURL = %v, want %v", cfg.Chain.RPCURL, "https://rpc.sepolia.org")
	}
	if cfg.Chain.ChainID != 11155111 {
		t.Errorf("ChainID = %v, want %v", cfg.Chain.ChainID, 11155111)
	}

	if cfg.GRPC.Host != "127.0.0.1" {
		t.Errorf("GRPC.Host = %v, want %v", cfg.GRPC.Host, "127.0.0.1")
	}
	if cfg.GRPC.Port != 50052 {
		t.Errorf("GRPC.Port = %v, want %v", cfg.GRPC.Port, 50052)
	}

	if !cfg.Log.Rotation {
		t.Error("Log.Rotation = false, want true")
	}
	if cfg.Log.FilePath != "/tmp/test.log" {
		t.Errorf("Log.FilePath = %v, want %v", cfg.Log.FilePath, "/tmp/test.log")
	}
	if cfg.Log.MaxSizeMB != 100 {
		t.Errorf("Log.MaxSizeMB = %v, want %v", cfg.Log.MaxSizeMB, 100)
	}
	if cfg.Log.MaxBackups != 10 {
		t.Errorf("Log.MaxBackups = %v, want %v", cfg.Log.MaxBackups, 10)
	}
	if cfg.Log.MaxAge != 30 {
		t.Errorf("Log.MaxAge = %v, want %v", cfg.Log.MaxAge, 30)
	}
	if !cfg.Log.Compress {
		t.Error("Log.Compress = false, want true")
	}
	if cfg.Log.Format != "terminal" {
		t.Errorf("Log.Format = %v, want %v", cfg.Log.Format, "terminal")
	}
	if cfg.Log.Verbosity != 3 {
		t.Errorf("Log.Verbosity = %v, want %v", cfg.Log.Verbosity, 3)
	}
	if cfg.Log.VModule != "" {
		t.Errorf("Log.VModule = %v, want %v", cfg.Log.VModule, "")
	}

	if cfg.Path != tmpFile.Name() {
		t.Errorf("Path = %v, want %v", cfg.Path, tmpFile.Name())
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.toml")
	if err == nil {
		t.Error("LoadConfig() should return error for nonexistent file")
	}
}

func TestSetConfigForTest(t *testing.T) {
	testCfg := &Config{
		Chain: ChainConfig{
			ChainCode: "ethereum",
			RPCURL:    "https://rpc.sepolia.org",
			ChainID:   11155111,
		},
		GRPC: GRPCConfig{
			Host: "localhost",
			Port: 9090,
		},
		Log: LogConfig{
			Rotation:  false,
			FilePath:  "/var/log/test.log",
			MaxSizeMB: 50,
		},
		Path: "/test/path/config.toml",
	}

	SetConfigForTest(testCfg)

	cfg := GetConfig()
	if cfg == nil {
		t.Fatal("GetConfig() returned nil after SetConfigForTest")
	}

	if cfg.Chain.ChainCode != "ethereum" {
		t.Errorf("Chain.ChainCode = %v, want %v", cfg.Chain.ChainCode, "ethereum")
	}
	if cfg.Chain.RPCURL != "https://rpc.sepolia.org" {
		t.Errorf("Chain.RPCURL = %v, want %v", cfg.Chain.RPCURL, "https://rpc.sepolia.org")
	}
	if cfg.Chain.ChainID != 11155111 {
		t.Errorf("Chain.ChainID = %v, want %v", cfg.Chain.ChainID, 11155111)
	}
	if cfg.GRPC.Host != "localhost" {
		t.Errorf("GRPC.Host = %v, want %v", cfg.GRPC.Host, "localhost")
	}
	if cfg.GRPC.Port != 9090 {
		t.Errorf("GRPC.Port = %v, want %v", cfg.GRPC.Port, 9090)
	}
	if cfg.Log.Rotation {
		t.Error("Log.Rotation = true, want false")
	}
	if cfg.Log.FilePath != "/var/log/test.log" {
		t.Errorf("Log.FilePath = %v, want %v", cfg.Log.FilePath, "/var/log/test.log")
	}
	if cfg.Log.MaxSizeMB != 50 {
		t.Errorf("Log.MaxSizeMB = %v, want %v", cfg.Log.MaxSizeMB, 50)
	}
	if cfg.Path != "/test/path/config.toml" {
		t.Errorf("Path = %v, want %v", cfg.Path, "/test/path/config.toml")
	}
}
