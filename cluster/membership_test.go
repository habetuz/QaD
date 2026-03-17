package cluster

import (
	"testing"

	"github.com/habetuz/qad/config"
	"github.com/rs/zerolog"
)

// TestNewManager verifies manager creation.
func TestNewManager(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		NodeName:    "test-node",
		ClusterPort: 0,  // Port 0 = OS picks any available port
		GRPCPort:    9876,
		SeedNodes:   []string{},
	}
	
	logger := zerolog.Nop()
	
	// Create manager
	manager, err := NewManager(cfg, logger)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	
	// Verify manager was created
	if manager == nil {
		t.Fatal("expected non-nil manager")
	}
	
	// Verify components are initialized
	if manager.list == nil {
		t.Error("memberlist not initialized")
	}
	if manager.delegate == nil {
		t.Error("delegate not initialized")
	}
	if manager.grpcPool == nil {
		t.Error("grpcPool not initialized")
	}
	if manager.hashRing == nil {
		t.Error("hashRing not initialized")
	}
	
	// Clean up
	defer manager.Leave()
}

// TestManager_Join_NoSeeds verifies starting a new cluster.
func TestManager_Join_NoSeeds(t *testing.T) {
	cfg := &config.Config{
		NodeName:    "test-node",
		ClusterPort: 0,
		GRPCPort:    9876,
		SeedNodes:   []string{},  // No seeds = new cluster
	}
	
	manager, err := NewManager(cfg, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Leave()
	
	// Join should succeed (starting new cluster)
	err = manager.Join()
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	
	// We should be the only member
	count := manager.GetMemberCount()
	if count != 1 {
		t.Errorf("expected 1 member, got %d", count)
	}
}

// TestManager_GetLocalNodeName verifies node name retrieval.
func TestManager_GetLocalNodeName(t *testing.T) {
	cfg := &config.Config{
		NodeName:    "my-custom-node",
		ClusterPort: 0,
		GRPCPort:    9876,
		SeedNodes:   []string{},
	}
	
	manager, err := NewManager(cfg, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Leave()
	
	if manager.GetLocalNodeName() != "my-custom-node" {
		t.Errorf("expected 'my-custom-node', got '%s'", manager.GetLocalNodeName())
	}
}

// TestManager_GetHashRing verifies hash ring access.
func TestManager_GetHashRing(t *testing.T) {
	cfg := &config.Config{
		NodeName:    "test-node",
		ClusterPort: 0,
		GRPCPort:    9876,
		SeedNodes:   []string{},
	}
	
	manager, err := NewManager(cfg, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Leave()
	
	hashRing := manager.GetHashRing()
	if hashRing == nil {
		t.Fatal("expected non-nil hash ring")
	}
}

// TestManager_GetGRPCPool verifies connection pool access.
func TestManager_GetGRPCPool(t *testing.T) {
	cfg := &config.Config{
		NodeName:    "test-node",
		ClusterPort: 0,
		GRPCPort:    9876,
		SeedNodes:   []string{},
	}
	
	manager, err := NewManager(cfg, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Leave()
	
	pool := manager.GetGRPCPool()
	if pool == nil {
		t.Fatal("expected non-nil gRPC pool")
	}
}

// TestManager_HealthCheck verifies health check data.
func TestManager_HealthCheck(t *testing.T) {
	cfg := &config.Config{
		NodeName:    "test-node",
		ClusterPort: 0,
		GRPCPort:    9876,
		SeedNodes:   []string{},
	}
	
	manager, err := NewManager(cfg, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Leave()
	
	err = manager.Join()
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	
	health := manager.HealthCheck()
	
	// Verify health data has expected fields
	if health["node_name"] != "test-node" {
		t.Errorf("expected node_name=test-node, got %v", health["node_name"])
	}
	
	if health["member_count"] != 1 {
		t.Errorf("expected member_count=1, got %v", health["member_count"])
	}
	
	// Verify members list exists
	if _, ok := health["members"]; !ok {
		t.Error("health check missing 'members' field")
	}
}

// TestManager_Leave verifies graceful shutdown.
func TestManager_Leave(t *testing.T) {
	cfg := &config.Config{
		NodeName:    "test-node",
		ClusterPort: 0,
		GRPCPort:    9876,
		SeedNodes:   []string{},
	}
	
	manager, err := NewManager(cfg, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	
	err = manager.Join()
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	
	// Leave should succeed
	err = manager.Leave()
	if err != nil {
		t.Errorf("Leave failed: %v", err)
	}
}

// TestGetOutboundIP verifies IP detection.
// This test might fail in environments without internet access.
func TestGetOutboundIP(t *testing.T) {
	ip := getOutboundIP()
	
	// Should return some IP (even if it's 127.0.0.1 in isolated environment)
	if ip == "" {
		t.Error("getOutboundIP returned empty string")
	}
	
	// Should be a valid IP format
	if len(ip) < 7 {  // Minimum: "1.1.1.1"
		t.Errorf("IP seems invalid: %s", ip)
	}
}