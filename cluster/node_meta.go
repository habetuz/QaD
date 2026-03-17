package cluster

import (
	"encoding/json"
	"fmt"
)

// NodeMeta represents the metadata that each node shares with the cluster.
type NodeMeta struct {
	NodeName string `json:"node_name"`
	GRPCAddr string `json:"grpc_addr"`
}

// Marshal converts the NodeMeta struct to a JSON byte slice suitable for
// transmitting over the network via memberlist's gossip protocol.
func (m *NodeMeta) Marshal() ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal node metadata: %w", err)
	}
	if len(data) > 512 {
		return nil, fmt.Errorf("metadata too large: %d bytes (max 512)", len(data))
	}
	return data, nil
}

// Unmarshal deserializes JSON bytes back into a NodeMeta struct.
func (m *NodeMeta) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, m); err != nil {
		return fmt.Errorf("failed to unmarshal node metadata: %w", err)
	}
	return nil
}
