package cluster

import (
	consistenthashring "github.com/habetuz/qad/consistent_hash_ring"
	"github.com/hashicorp/memberlist"
	"github.com/rs/zerolog"
)

// EventDelegate handles memberlist cluster events (joins, leaves, updates).
type EventDelegate struct {
	logger zerolog.Logger

	hashRing *consistenthashring.ConsistentHashRing

	grpcPool *GRPCPool

	localNodeName string
}

// NewEventDelegate creates a new event handler for cluster membership events.
func NewEventDelegate(
	logger zerolog.Logger,
	hashRing *consistenthashring.ConsistentHashRing,
	grpcPool *GRPCPool,
	localNodeName string,
) *EventDelegate {
	return &EventDelegate{
		logger:        logger,
		hashRing:      hashRing,
		grpcPool:      grpcPool,
		localNodeName: localNodeName,
	}
}

// NotifyJoin is called by memberlist when a node joins the cluster.
// This method implements part of the EventDelegate interface.
//
// We must:
// 1. Parse the node's metadata to get its gRPC address
// 2. Add the node to our hash ring (unless it's us)
// 3. Create a gRPC connection to the node
//
// Parameters:
//   - node: Information about the joining node (name, address, metadata)
//
// Interface Implementation Note:
// This method signature MUST match memberlist.EventDelegate.NotifyJoin
// exactly, or the interface won't be satisfied.
func (e *EventDelegate) NotifyJoin(node *memberlist.Node) {
	// Log the join event with structured fields for better observability
	e.logger.Info().
		Str("node", node.Name).
		Str("addr", node.Address()).
		Msg("Node joined cluster")

	var meta NodeMeta
	if err := meta.Unmarshal(node.Meta); err != nil {
		e.logger.Error().
			Err(err).
			Str("node", node.Name).
			Msg("Failed to unmarshal node metadata on join")
		return
	}

	if node.Name == e.localNodeName {
		e.logger.Debug().
			Str("node", node.Name).
			Msg("Skipping self in NotifyJoin")
		return
	}

	e.hashRing.AddNode(node.Name)

	// Create a gRPC connection to the new node
	// This is done lazily - the connection is established on first use
	if err := e.grpcPool.AddConnection(node.Name, meta.GRPCAddr); err != nil {
		e.logger.Error().
			Err(err).
			Str("node", node.Name).
			Str("grpc_addr", meta.GRPCAddr).
			Msg("Failed to add gRPC connection for joined node")
	}
}

// NotifyLeave is called by memberlist when a node gracefully leaves the cluster.
func (e *EventDelegate) NotifyLeave(node *memberlist.Node) {
	e.logger.Info().
		Str("node", node.Name).
		Str("addr", node.Address()).
		Msg("Node left cluster")

	if node.Name == e.localNodeName {
		e.logger.Debug().
			Str("node", node.Name).
			Msg("Skipping self in NotifyLeave")
		return
	}

	e.hashRing.RemoveNode(node.Name)

	if err := e.grpcPool.RemoveConnection(node.Name); err != nil {
		e.logger.Error().
			Err(err).
			Str("node", node.Name).
			Msg("Failed to remove gRPC connection for left node")
	}
}

// NotifyUpdate is called by memberlist when a node's metadata changes.
func (e *EventDelegate) NotifyUpdate(node *memberlist.Node) {
	e.logger.Info().
		Str("node", node.Name).
		Str("addr", node.Address()).
		Msg("Node metadata updated")

	var meta NodeMeta
	if err := meta.Unmarshal(node.Meta); err != nil {
		e.logger.Error().
			Err(err).
			Str("node", node.Name).
			Msg("Failed to unmarshal node metadata on update")
		return
	}

	if node.Name == e.localNodeName {
		e.logger.Debug().
			Str("node", node.Name).
			Msg("Skipping self in NotifyUpdate")
		return
	}

	if err := e.grpcPool.UpdateConnection(node.Name, meta.GRPCAddr); err != nil {
		e.logger.Error().
			Err(err).
			Str("node", node.Name).
			Str("grpc_addr", meta.GRPCAddr).
			Msg("Failed to update gRPC connection")
	}
}
