package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Chain ChainConfig `toml:"chain"`
	GRPC  GRPCConfig  `toml:"grpc"`
	Log   LogConfig   `toml:"log"`
	Path  string
}

type ChainConfig struct {
	ChainCode string `toml:"chain_code"`
	RPCURL    string `toml:"rpc_url"`
	ChainID   uint64 `toml:"chain_id"`
}

type GRPCConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type LogConfig struct {
	Rotation   bool   `toml:"rotation"`
	FilePath   string `toml:"file_path"`
	MaxSizeMB  int    `toml:"max_size_mb"`
	MaxBackups int    `toml:"max_backups"`
	MaxAge     int    `toml:"max_age"`
	Compress   bool   `toml:"compress"`
	Format     string `toml:"format"`
	Verbosity  int    `toml:"verbosity"`
	VModule    string `toml:"vmodule"`
}

const (
	DEFAULT_PATH = "config/config.toml"
)

var (
	cfg *Config
)

func LoadConfig(path string) (*Config, error) {
	configData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg = &Config{}
	if err := toml.Unmarshal(configData, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	cfg.Path = path

	return cfg, nil
}

func GetConfig() *Config {
	return cfg
}

func SetConfigForTest(c *Config) {
	cfg = c
}
