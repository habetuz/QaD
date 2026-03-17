package cluster

import (
	"github.com/rs/zerolog"
)

// MetadataDelegate implements the memberlist.Delegate interface to provide
// node metadata (like gRPC address) to other cluster members.
type MetadataDelegate struct {
	logger   zerolog.Logger
	nodeMeta NodeMeta
}

// NewMetadataDelegate creates a new delegate for providing node metadata.
func NewMetadataDelegate(logger zerolog.Logger, nodeMeta NodeMeta) *MetadataDelegate {
	return &MetadataDelegate{
		logger:   logger,
		nodeMeta: nodeMeta,
	}
}

// NodeMeta returns the metadata for this node.
// This is called by memberlist when broadcasting alive messages.
func (d *MetadataDelegate) NodeMeta(limit int) []byte {
	data, err := d.nodeMeta.Marshal()
	if err != nil {
		d.logger.Error().
			Err(err).
			Msg("Failed to marshal node metadata")
		return []byte{}
	}

	if len(data) > limit {
		d.logger.Warn().
			Int("size", len(data)).
			Int("limit", limit).
			Msg("Node metadata exceeds limit, truncating")
		return data[:limit]
	}

	return data
}

// NotifyMsg is called when a user-data message is received.
// We don't use custom user messages, so this is a no-op.
func (d *MetadataDelegate) NotifyMsg([]byte) {
	// No-op: we don't use custom user messages
}

// GetBroadcasts is called when user data messages can be broadcast.
// We don't use broadcasts, so return empty.
func (d *MetadataDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	// No-op: we don't use broadcasts
	return nil
}

// LocalState is used for a TCP Push/Pull.
// We don't need to send additional state, so return empty.
func (d *MetadataDelegate) LocalState(join bool) []byte {
	// No-op: we don't send additional state
	return []byte{}
}

// MergeRemoteState is invoked after a TCP Push/Pull.
// We don't need to merge additional state, so this is a no-op.
func (d *MetadataDelegate) MergeRemoteState(buf []byte, join bool) {
	// No-op: we don't merge additional state
}
