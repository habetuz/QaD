package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

type Member struct {
	Name string
	Host string
}

type EvictionAlgorithm string

const (
	FIFO EvictionAlgorithm = "FIFO"
	LRU  EvictionAlgorithm = "LRU"
	NONE EvictionAlgorithm = "NONE"
)

type Config struct {
	LogLevel          zerolog.Level
	HTTPPort          uint32
	GRPCPort          uint32
	Members           []Member
	StorageSize       int
	EvictionAlgorithm EvictionAlgorithm
	NodeName          string
	ClusterPort       int
	SeedNodes         []string
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

	if v := os.Getenv("STORAGE_SIZE"); v != "" {
		size, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid STORAGE_SIZE %q: %w", v, err)
		}
		cfg.StorageSize = size
	}

	if v := os.Getenv("EVICTION_ALGORITHM"); v != "" {
		switch EvictionAlgorithm(v) {
		case FIFO, LRU:
			cfg.EvictionAlgorithm = EvictionAlgorithm(v)
		default:
			return nil, fmt.Errorf("invalid EVICTION_ALGORITHM %q: must be one of FIFO, LRU", v)
		}
	}

	for i := 0; ; i++ {
		name := os.Getenv(fmt.Sprintf("MEMBER_%d_NAME", i))
		host := os.Getenv(fmt.Sprintf("MEMBER_%d_HOST", i))
		if name == "" && host == "" {
			break
		}
		cfg.Members = append(cfg.Members, Member{Name: name, Host: host})
	}

	if v := os.Getenv("NODE_NAME"); v != "" {
		cfg.NodeName = v
	} else {
		cfg.NodeName = generateNodeID()
	}

	if v := os.Getenv("CLUSTER_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("Invalid CLUSTER_PORT %q: %w", v, err)
		}
		cfg.ClusterPort = port
	}

	if v := os.Getenv("SEED_NODES"); v != "" {
		// strings.Split breaks "a,b,c" into ["a", "b", "c"]
		seeds := strings.Split(v, ",")

		// Trim whitespace from each seed node address
		for i := range seeds {
			seeds[i] = strings.TrimSpace(seeds[i])
		}

		cfg.SeedNodes = seeds
	}

	return &cfg, nil
}

func generateNodeID() string {
	timestamp := time.Now().Unix()
	randomBytes := make([]byte, 3)

	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("node-%d-000000", timestamp)
	}

	randomHex := hex.EncodeToString(randomBytes)
	return fmt.Sprintf("node-%d-%s", timestamp, randomHex)
}

func defaultConfig() Config {
	return Config{
		LogLevel:          zerolog.InfoLevel,
		HTTPPort:          8080,
		GRPCPort:          9876,
		StorageSize:       1e+9,
		EvictionAlgorithm: FIFO,
		NodeName:          generateNodeID(),
		ClusterPort:       7946,
		SeedNodes:         []string{},
	}
}
