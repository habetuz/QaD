package config

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// TestLoad_Defaults verifies that the default configuration is loaded
// when no environment variables are set.
func TestLoad_Defaults(t *testing.T) {
	// Clear any environment variables that might interfere
	clearConfigEnvVars(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify default values
	if cfg.LogLevel != zerolog.InfoLevel {
		t.Errorf("expected LogLevel=%v, got %v", zerolog.InfoLevel, cfg.LogLevel)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("expected HTTPPort=8080, got %d", cfg.HTTPPort)
	}
	if cfg.GRPCPort != 9876 {
		t.Errorf("expected GRPCPort=9876, got %d", cfg.GRPCPort)
	}
	if cfg.StorageSize != 1e+9 {
		t.Errorf("expected StorageSize=1e+9, got %d", cfg.StorageSize)
	}
	if cfg.EvictionAlgorithm != FIFO {
		t.Errorf("expected EvictionAlgorithm=FIFO, got %s", cfg.EvictionAlgorithm)
	}
	if cfg.ClusterPort != 7946 {
		t.Errorf("expected ClusterPort=7946, got %d", cfg.ClusterPort)
	}
	if len(cfg.SeedNodes) != 0 {
		t.Errorf("expected empty SeedNodes, got %v", cfg.SeedNodes)
	}

	// NodeName should be auto-generated
	if cfg.NodeName == "" {
		t.Error("expected auto-generated NodeName, got empty string")
	}
	if !strings.HasPrefix(cfg.NodeName, "node-") {
		t.Errorf("expected NodeName to start with 'node-', got %q", cfg.NodeName)
	}
}

// TestLoad_NodeName verifies that NODE_NAME environment variable is respected.
func TestLoad_NodeName(t *testing.T) {
	clearConfigEnvVars(t)

	// Set NODE_NAME
	t.Setenv("NODE_NAME", "my-custom-node-name")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.NodeName != "my-custom-node-name" {
		t.Errorf("expected NodeName='my-custom-node-name', got %q", cfg.NodeName)
	}
}

// TestLoad_ClusterPort verifies that CLUSTER_PORT environment variable is respected.
func TestLoad_ClusterPort(t *testing.T) {
	clearConfigEnvVars(t)

	// Set CLUSTER_PORT
	t.Setenv("CLUSTER_PORT", "8888")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.ClusterPort != 8888 {
		t.Errorf("expected ClusterPort=8888, got %d", cfg.ClusterPort)
	}
}

// TestLoad_ClusterPort_Invalid verifies that invalid CLUSTER_PORT returns an error.
func TestLoad_ClusterPort_Invalid(t *testing.T) {
	clearConfigEnvVars(t)

	// Set invalid CLUSTER_PORT
	t.Setenv("CLUSTER_PORT", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid CLUSTER_PORT, got nil")
	}

	// Verify the error message mentions CLUSTER_PORT
	if !strings.Contains(err.Error(), "CLUSTER_PORT") {
		t.Errorf("expected error to mention CLUSTER_PORT, got: %v", err)
	}
}

// TestLoad_SeedNodes verifies that SEED_NODES environment variable is parsed correctly.
func TestLoad_SeedNodes(t *testing.T) {
	clearConfigEnvVars(t)

	// Set SEED_NODES with comma-separated values
	t.Setenv("SEED_NODES", "10.0.1.5:7946,10.0.1.6:7946,10.0.1.7:7946")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	expectedSeeds := []string{"10.0.1.5:7946", "10.0.1.6:7946", "10.0.1.7:7946"}
	if len(cfg.SeedNodes) != len(expectedSeeds) {
		t.Fatalf("expected %d seed nodes, got %d", len(expectedSeeds), len(cfg.SeedNodes))
	}

	for i, expected := range expectedSeeds {
		if cfg.SeedNodes[i] != expected {
			t.Errorf("seed node %d: expected %q, got %q", i, expected, cfg.SeedNodes[i])
		}
	}
}

// TestLoad_SeedNodes_WithWhitespace verifies that whitespace around seed nodes is trimmed.
func TestLoad_SeedNodes_WithWhitespace(t *testing.T) {
	clearConfigEnvVars(t)

	// Set SEED_NODES with extra whitespace
	t.Setenv("SEED_NODES", "10.0.1.5:7946 , 10.0.1.6:7946  ,  10.0.1.7:7946")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify whitespace is trimmed
	expectedSeeds := []string{"10.0.1.5:7946", "10.0.1.6:7946", "10.0.1.7:7946"}
	for i, expected := range expectedSeeds {
		if cfg.SeedNodes[i] != expected {
			t.Errorf("seed node %d: expected %q, got %q (whitespace not trimmed?)",
				i, expected, cfg.SeedNodes[i])
		}
	}
}

// TestGenerateNodeID verifies that generateNodeID creates valid, unique IDs.
func TestGenerateNodeID(t *testing.T) {
	// Generate multiple IDs
	id1 := generateNodeID()
	id2 := generateNodeID()

	// Verify format
	if !strings.HasPrefix(id1, "node-") {
		t.Errorf("expected ID to start with 'node-', got %q", id1)
	}

	// Verify structure: "node-<timestamp>-<hex>"
	parts := strings.Split(id1, "-")
	if len(parts) != 3 {
		t.Errorf("expected ID to have 3 parts separated by '-', got %d parts: %q", len(parts), id1)
	}

	// Verify hex part has correct length (3 bytes = 6 hex characters)
	if len(parts) == 3 && len(parts[2]) != 6 {
		t.Errorf("expected hex part to be 6 characters, got %d: %q", len(parts[2]), parts[2])
	}

	// Verify uniqueness (very unlikely to collide)
	if id1 == id2 {
		t.Errorf("generated IDs should be unique, but got identical: %q", id1)
	}
}

// TestGenerateNodeID_Format verifies the exact format of generated IDs.
func TestGenerateNodeID_Format(t *testing.T) {
	id := generateNodeID()

	// Split by hyphen
	parts := strings.Split(id, "-")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d: %q", len(parts), id)
	}

	// Verify first part is "node"
	if parts[0] != "node" {
		t.Errorf("expected first part to be 'node', got %q", parts[0])
	}

	// Verify second part is a number (timestamp)
	for _, ch := range parts[1] {
		if ch < '0' || ch > '9' {
			t.Errorf("timestamp part contains non-digit: %q", parts[1])
			break
		}
	}

	// Verify third part is hexadecimal (0-9, a-f)
	for _, ch := range parts[2] {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("hex part contains invalid character %q: %q", string(ch), parts[2])
			break
		}
	}
}

// clearConfigEnvVars clears all configuration-related environment variables
// to ensure tests start with a clean slate.
func clearConfigEnvVars(t *testing.T) {
	t.Helper() // Marks this as a test helper function

	// Clear all config-related env vars
	vars := []string{
		"LOG_LEVEL", "HTTP_PORT", "GRPC_PORT", "STORAGE_SIZE",
		"EVICTION_ALGORITHM", "NODE_NAME", "CLUSTER_PORT", "SEED_NODES",
	}

	for _, v := range vars {
		os.Unsetenv(v)
	}

	// Clear MEMBER_* variables (0-9 should be enough)
	for i := 0; i < 10; i++ {
		os.Unsetenv(fmt.Sprintf("MEMBER_%d_NAME", i))
		os.Unsetenv(fmt.Sprintf("MEMBER_%d_HOST", i))
	}
}
