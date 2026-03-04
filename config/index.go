package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

type Member struct {
	Name string
	Host string
}

type Config struct {
	LogLevel zerolog.Level
	HTTPPort uint32
	GRPCPort uint32
	Members  []Member
}

func Load() (*Config, error) {
	// Load .env file if present, ignore error if it doesn't exist
	_ = godotenv.Load()

	cfg := defaultConfig()

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		level, err := zerolog.ParseLevel(v)
		if err != nil {
			return nil, fmt.Errorf("invalid LOG_LEVEL %q: %w", v, err)
		}
		cfg.LogLevel = level
	}

	if v := os.Getenv("HTTP_PORT"); v != "" {
		port, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid HTTP_PORT %q: %w", v, err)
		}
		cfg.HTTPPort = uint32(port)
	}

	if v := os.Getenv("GRPC_PORT"); v != "" {
		port, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid GRPC_PORT %q: %w", v, err)
		}
		cfg.GRPCPort = uint32(port)
	}

	for i := 0; ; i++ {
		name := os.Getenv(fmt.Sprintf("MEMBER_%d_NAME", i))
		host := os.Getenv(fmt.Sprintf("MEMBER_%d_HOST", i))
		if name == "" && host == "" {
			break
		}
		cfg.Members = append(cfg.Members, Member{Name: name, Host: host})
	}

	return &cfg, nil
}

func defaultConfig() Config {
	return Config{
		LogLevel: zerolog.InfoLevel,
		HTTPPort: 8080,
		GRPCPort: 9876,
	}
}
