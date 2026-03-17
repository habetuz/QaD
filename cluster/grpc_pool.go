package cluster

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCPool manages a pool of gRPC connections to other nodes in the cluster.
type GRPCPool struct {
	mu sync.RWMutex

	connections map[string]*poolEntry

	logger zerolog.Logger
}

// poolEntry holds a gRPC connection and metadata about it.
type poolEntry struct {
	conn *grpc.ClientConn

	addr string

	once sync.Once
}

// NewGRPCPool creates a new connection pool.
func NewGRPCPool(logger zerolog.Logger) *GRPCPool {
	return &GRPCPool{
		connections: make(map[string]*poolEntry),
		logger:      logger,
	}
}

// AddConnection registers a node in the pool.
func (p *GRPCPool) AddConnection(nodeName, addr string) error {
	if nodeName == "" {
		return fmt.Errorf("node name cannot be empty")
	}
	if addr == "" {
		return fmt.Errorf("address cannot be empty for node %s", nodeName)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Create a pool entry with no connection yet (lazy initialization)
	p.connections[nodeName] = &poolEntry{
		addr: addr,
	}

	p.logger.Debug().
		Str("node", nodeName).
		Str("addr", addr).
		Msg("Added node to gRPC pool")

	return nil
}

// GetConnection retrieves a gRPC connection for the specified node.
func (p *GRPCPool) GetConnection(nodeName string) (*grpc.ClientConn, error) {

	p.mu.RLock()
	entry, exists := p.connections[nodeName]
	p.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("node %s not found in connection pool", nodeName)
	}

	// Use sync.Once to ensure connection is created exactly once
	var dialErr error
	entry.once.Do(func() {
		
		conn, err := grpc.NewClient(
			entry.addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			dialErr = fmt.Errorf("failed to dial %s: %w", entry.addr, err)
			p.logger.Error().
				Err(err).
				Str("node", nodeName).
				Str("addr", entry.addr).
				Msg("Failed to create gRPC connection")
			return
		}

		entry.conn = conn
		p.logger.Debug().
			Str("node", nodeName).
			Str("addr", entry.addr).
			Msg("Created gRPC connection")
	})

	if dialErr != nil {
		return nil, dialErr
	}

	if entry.conn == nil {
		return nil, fmt.Errorf("connection for node %s is nil", nodeName)
	}

	return entry.conn, nil
}

// UpdateConnection closes the old connection and prepares for a new one.
func (p *GRPCPool) UpdateConnection(nodeName, newAddr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry, exists := p.connections[nodeName]
	if !exists {
		return fmt.Errorf("node %s not found in connection pool", nodeName)
	}

	// Close old connection if it exists
	if entry.conn != nil {
		if err := entry.conn.Close(); err != nil {
			p.logger.Warn().
				Err(err).
				Str("node", nodeName).
				Msg("Error closing old gRPC connection during update")
		}
	}

	// Create new entry with updated address
	// The connection will be created lazily on next GetConnection call
	p.connections[nodeName] = &poolEntry{
		addr: newAddr,
	}

	p.logger.Info().
		Str("node", nodeName).
		Str("old_addr", entry.addr).
		Str("new_addr", newAddr).
		Msg("Updated gRPC connection address")

	return nil
}

// RemoveConnection closes and removes a connection from the pool.
func (p *GRPCPool) RemoveConnection(nodeName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry, exists := p.connections[nodeName]
	if !exists {
		// Not an error - node might have been removed already
		return fmt.Errorf("node %s not found in connection pool", nodeName)
	}

	// Close connection if it was created
	if entry.conn != nil {
		if err := entry.conn.Close(); err != nil {
			p.logger.Warn().
				Err(err).
				Str("node", nodeName).
				Msg("Error closing gRPC connection during removal")
		}
	}

	// Remove from map
	delete(p.connections, nodeName)

	p.logger.Debug().
		Str("node", nodeName).
		Msg("Removed node from gRPC pool")

	return nil
}

// Close closes all gRPC connections in the pool.
func (p *GRPCPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close all connections
	for nodeName, entry := range p.connections {
		if entry.conn != nil {
			if err := entry.conn.Close(); err != nil {
				p.logger.Warn().
					Err(err).
					Str("node", nodeName).
					Msg("Error closing gRPC connection during pool shutdown")
			}
		}
	}

	// Clear the map
	p.connections = make(map[string]*poolEntry)

	p.logger.Info().Msg("Closed all gRPC connections")

	return nil
}

// GetAllNodeNames returns a list of all node names currently in the pool.
func (p *GRPCPool) GetAllNodeNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	names := make([]string, 0, len(p.connections))
	for name := range p.connections {
		names = append(names, name)
	}

	return names
}
