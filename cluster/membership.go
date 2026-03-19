package cluster

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/habetuz/qad/config"
	consistenthashring "github.com/habetuz/qad/consistent_hash_ring"
	"github.com/hashicorp/memberlist"
	"github.com/rs/zerolog"
)

// Manager orchestrates cluster membership using HashiCorp's memberlist library.
// It handles node discovery, failure detection, and cluster state synchronization.
//
// The Manager integrates several components:
//   - memberlist: Gossip-based cluster membership (failure detection, discovery)
//   - EventDelegate: Our custom handler for join/leave/update events
//   - GRPCPool: Connection pool for inter-node communication
//   - ConsistentHashRing: Determines which node owns which keys
//
// Lifecycle:
//  1. NewManager(): Create and configure
//  2. Join(): Join the cluster (or start a new one)
//  3. Use GetMembers(), GetGRPCPool(), etc.
//  4. Leave(): Gracefully exit the cluster
type Manager struct {
	// list is the memberlist instance that handles the gossip protocol
	list *memberlist.Memberlist

	// delegate handles cluster events (join/leave/update)
	delegate *EventDelegate

	// grpcPool manages gRPC connections to cluster members
	grpcPool *GRPCPool

	// hashRing determines key ownership in the cluster
	hashRing *consistenthashring.ConsistentHashRing

	// logger for debugging and operational visibility
	logger zerolog.Logger

	// config stores the original configuration for reference
	config *config.Config
}

// NewManager creates a new cluster membership manager.
func NewManager(cfg *config.Config, logger zerolog.Logger) (*Manager, error) {

	hashRing := consistenthashring.NewRing(150)

	grpcPool := NewGRPCPool(logger)

	delegate := NewEventDelegate(logger, hashRing, grpcPool, cfg.NodeName, cfg.GRPCPort)

	mlConfig := memberlist.DefaultLocalConfig()
	mlConfig.Logger = log.New(&ZerologWriter{logger: logger}, "", 0)

	mlConfig.Name = cfg.NodeName
	mlConfig.BindPort = cfg.ClusterPort
	mlConfig.Events = delegate

	// Set up node metadata that will be gossiped to other nodes
	// This tells other nodes how to communicate with us via gRPC
	grpcAddr := fmt.Sprintf("%s:%d", getOutboundIP(), cfg.GRPCPort)
	nodeMeta := NodeMeta{
		NodeName: cfg.NodeName,
		GRPCAddr: grpcAddr,
	}

	// Create metadata delegate to provide our gRPC address to other nodes
	metadataDelegate := NewMetadataDelegate(logger, nodeMeta)
	mlConfig.Delegate = metadataDelegate

	// Create the memberlist instance
	list, err := memberlist.Create(mlConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create memberlist: %w", err)
	}

	// Add ourselves to the hash ring immediately
	// This ensures the ring is never empty and we can handle requests for our own keys
	hashRing.AddNode(cfg.NodeName)

	logger.Info().
		Str("node_name", cfg.NodeName).
		Int("cluster_port", cfg.ClusterPort).
		Str("grpc_addr", grpcAddr).
		Msg("Created cluster membership manager")

	return &Manager{
		list:     list,
		delegate: delegate,
		grpcPool: grpcPool,
		hashRing: hashRing,
		logger:   logger,
		config:   cfg,
	}, nil
}

// Join attempts to join an existing cluster by contacting seed nodes.
func (m *Manager) Join() error {

	if len(m.config.SeedNodes) == 0 {
		m.logger.Info().Msg("No seed nodes configured, starting new cluster")
		return nil
	}

	// Attempt to join existing cluster by contacting seed nodes
	n, err := m.list.Join(m.config.SeedNodes)
	if err != nil {
		return fmt.Errorf("failed to join cluster: %w", err)
	}

	m.logger.Info().
		Int("nodes_contacted", n).
		Strs("seed_nodes", m.config.SeedNodes).
		Msg("Joined cluster")

	return nil
}

// Leave gracefully removes this node from the cluster.
func (m *Manager) Leave() error {

	if err := m.list.Leave((30 * time.Second)); err != nil {
		return fmt.Errorf("failed to leave cluster: %w", err)
	}

	if err := m.list.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown memberlist: %w", err)
	}

	if err := m.grpcPool.Close(); err != nil {
		m.logger.Warn().Err(err).Msg("Error closing gRPC pool")
	}

	m.logger.Info().Msg("Left cluster and shut down membership manager")

	return nil
}

// GetMembers returns a list of all nodes currently in the cluster.
func (m *Manager) GetMembers() []*memberlist.Node {
	return m.list.Members()
}

// GetMemberCount returns the number of nodes in the cluster.
func (m *Manager) GetMemberCount() int {
	return m.list.NumMembers()
}

// GetGRPCPool returns the gRPC connection pool.
func (m *Manager) GetGRPCPool() *GRPCPool {
	return m.grpcPool
}

// GetHashRing returns the consistent hash ring.
func (m *Manager) GetHashRing() *consistenthashring.ConsistentHashRing {
	return m.hashRing
}

// GetLocalNodeName returns this node's name.
func (m *Manager) GetLocalNodeName() string {
	return m.config.NodeName
}

// getOutboundIP returns this machine's preferred outbound IP address.
// This is used to determine what IP address to advertise to other nodes.
//
// The function works by:
// 1. Opening a UDP connection to a public IP (doesn't actually send data)
// 2. Checking what local IP the OS chose for that connection
// 3. Returning that IP
//
// This is a common technique for determining the "public-facing" IP
// on machines with multiple network interfaces.
//
// Returns:
//   - string: IP address (e.g., "10.0.1.5" or "192.168.1.10")
func getOutboundIP() string {

	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}

// HealthCheck returns information about cluster health.
// This is useful for monitoring and debugging.
//
// Returns:
//   - map[string]interface{}: Health information
func (m *Manager) HealthCheck() map[string]interface{} {
	members := m.GetMembers()

	// Extract member information
	memberInfo := make([]map[string]string, len(members))
	for i, member := range members {
		memberInfo[i] = map[string]string{
			"name":    member.Name,
			"address": member.Address(),
			"state":   strconv.Itoa(int(member.State)),
		}
	}

	return map[string]interface{}{
		"node_name":       m.config.NodeName,
		"member_count":    len(members),
		"members":         memberInfo,
		"hash_ring_nodes": m.grpcPool.GetAllNodeNames(),
	}
}
