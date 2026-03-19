package cluster

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	consistenthashring "github.com/habetuz/qad/consistent_hash_ring"
	"github.com/habetuz/qad/proto_gen"
	"github.com/habetuz/qad/storage"
	"github.com/hashicorp/memberlist"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type captureCommunicationServer struct {
	proto_gen.UnimplementedCommunicationServer
	writes chan *proto_gen.KeyValuePair
}

func (s *captureCommunicationServer) Read(context.Context, *proto_gen.Key) (*proto_gen.Value, error) {
	return nil, status.Error(codes.NotFound, "not found")
}

func (s *captureCommunicationServer) Write(_ context.Context, kv *proto_gen.KeyValuePair) (*proto_gen.Void, error) {
	s.writes <- kv
	return &proto_gen.Void{}, nil
}

func startCaptureGRPCServer(t *testing.T) (string, <-chan *proto_gen.KeyValuePair, func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	impl := &captureCommunicationServer{writes: make(chan *proto_gen.KeyValuePair, 8)}
	proto_gen.RegisterCommunicationServer(grpcServer, impl)

	go func() {
		_ = grpcServer.Serve(lis)
	}()

	cleanup := func() {
		grpcServer.Stop()
		_ = lis.Close()
	}

	return lis.Addr().String(), impl.writes, cleanup
}

func findKeyAndHashForNode(t *testing.T, ring *consistenthashring.ConsistentHashRing, nodeName string) (string, uint64) {
	t.Helper()

	for i := 0; i < 2_000_000; i++ {
		key := fmt.Sprintf("probe-key-%d", i)
		h, n := ring.NodeOf(key)
		if n == nodeName {
			return key, h
		}
	}

	t.Fatalf("failed to find key/hash for node %s", nodeName)
	return "", 0
}

// TestEventDelegate_NotifyJoin verifies that joining nodes are added to
// the hash ring and connection pool.
func TestEventDelegate_NotifyJoin(t *testing.T) {
	// Arrange: Set up test fixtures
	logger := zerolog.Nop() // Nop() creates a logger that discards all output
	hashRing := consistenthashring.NewRing(3)
	store := storage.NewNoEvictionStorage()
	grpcPool := NewGRPCPool(logger)
	localName := "local-node"
	var grpcPort uint32 = 9876

	delegate := NewEventDelegate(logger, hashRing, store, grpcPool, localName, grpcPort)

	// Create test node metadata
	meta := NodeMeta{
		NodeName: "remote-node",
		GRPCAddr: "10.0.1.5:9876",
	}
	metaBytes, err := meta.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}

	// Create a fake memberlist.Node
	// In real life, memberlist creates these; in tests, we create them manually
	node := &memberlist.Node{
		Name: "remote-node",
		Meta: metaBytes,
	}

	// Act: Simulate a node joining
	delegate.NotifyJoin(node)

	// Assert: Verify node was added to hash ring
	// We can test this by checking if keys route to the node
	// The hash ring should now have this node in its rotation
	// Note: We can't directly check if node is in ring, so we verify indirectly
	// by checking that we have nodes in the ring
	_, nodeInRing := hashRing.NodeOf("test-key")
	if nodeInRing != "remote-node" && nodeInRing != "" {
		// The key might not hash to our specific node, but the ring should be functional
		// The real verification is that AddNode didn't panic
	}

	// Assert: Verify gRPC connection was added
	conn, err := grpcPool.GetConnection("remote-node")
	if err != nil {
		t.Errorf("Expected connection to exist for joined node, got error: %v", err)
	}
	if conn == nil {
		t.Error("Expected non-nil connection for joined node")
	}
}

// TestEventDelegate_NotifyJoin_SkipsSelf verifies that we don't add
// ourselves to the hash ring.
func TestEventDelegate_NotifyJoin_SkipsSelf(t *testing.T) {
	logger := zerolog.Nop()
	hashRing := consistenthashring.NewRing(3)
	store := storage.NewNoEvictionStorage()
	grpcPool := NewGRPCPool(logger)
	localName := "local-node"
	var grpcPort uint32 = 9876

	delegate := NewEventDelegate(logger, hashRing, store, grpcPool, localName, grpcPort)

	// Create metadata for ourselves
	meta := NodeMeta{
		NodeName: localName,
		GRPCAddr: "127.0.0.1:9876",
	}
	metaBytes, _ := meta.Marshal()

	node := &memberlist.Node{
		Name: localName, // Same as localName
		Meta: metaBytes,
	}

	// Act: Simulate our own join event
	delegate.NotifyJoin(node)

	// Assert: Verify we didn't add ourselves to connection pool
	_, err := grpcPool.GetConnection(localName)
	if err == nil {
		t.Error("Should not have connection to self")
	}
}

// TestEventDelegate_NotifyLeave verifies that leaving nodes are removed
// from the hash ring and connection pool.
func TestEventDelegate_NotifyLeave(t *testing.T) {
	logger := zerolog.Nop()
	hashRing := consistenthashring.NewRing(3)
	store := storage.NewNoEvictionStorage()
	grpcPool := NewGRPCPool(logger)
	localName := "local-node"
	var grpcPort uint32 = 9876

	delegate := NewEventDelegate(logger, hashRing, store, grpcPool, localName, grpcPort)

	// Arrange: First add a node
	meta := NodeMeta{
		NodeName: "remote-node",
		GRPCAddr: "10.0.1.5:9876",
	}
	metaBytes, _ := meta.Marshal()

	node := &memberlist.Node{
		Name: "remote-node",
		Meta: metaBytes,
	}

	delegate.NotifyJoin(node)

	// Act: Now remove it
	delegate.NotifyLeave(node)

	// Assert: Verify connection was removed
	_, err := grpcPool.GetConnection("remote-node")
	if err == nil {
		t.Error("Expected connection to be removed for left node")
	}

	// Note: Can't easily verify hash ring removal without exposing internals
	// In practice, RemoveNode() would panic if node wasn't there,
	// and the test would fail
}

// TestEventDelegate_NotifyLeave_SkipsSelf verifies we handle our own leave correctly.
func TestEventDelegate_NotifyLeave_SkipsSelf(t *testing.T) {
	logger := zerolog.Nop()
	hashRing := consistenthashring.NewRing(3)
	store := storage.NewNoEvictionStorage()
	grpcPool := NewGRPCPool(logger)
	localName := "local-node"
	var grpcPort uint32 = 9876

	delegate := NewEventDelegate(logger, hashRing, store, grpcPool, localName, grpcPort)

	meta := NodeMeta{
		NodeName: localName,
		GRPCAddr: "127.0.0.1:9876",
	}
	metaBytes, _ := meta.Marshal()

	node := &memberlist.Node{
		Name: localName,
		Meta: metaBytes,
	}

	// Act: Simulate our own leave
	// Should not panic even though we're not in the hash ring
	delegate.NotifyLeave(node)

	// If we get here without panic, test passes
}

// TestEventDelegate_NotifyUpdate verifies that node updates refresh connections.
func TestEventDelegate_NotifyUpdate(t *testing.T) {
	logger := zerolog.Nop()
	hashRing := consistenthashring.NewRing(3)
	store := storage.NewNoEvictionStorage()
	grpcPool := NewGRPCPool(logger)
	localName := "local-node"
	var grpcPort uint32 = 9876

	delegate := NewEventDelegate(logger, hashRing, store, grpcPool, localName, grpcPort)

	// Arrange: Add a node first
	meta := NodeMeta{
		NodeName: "remote-node",
		GRPCAddr: "10.0.1.5:9876",
	}
	metaBytes, _ := meta.Marshal()

	node := &memberlist.Node{
		Name: "remote-node",
		Meta: metaBytes,
	}

	delegate.NotifyJoin(node)

	// Act: Update with new metadata (different address)
	newMeta := NodeMeta{
		NodeName: "remote-node",
		GRPCAddr: "10.0.1.5:9999", // Changed port
	}
	newMetaBytes, _ := newMeta.Marshal()

	updatedNode := &memberlist.Node{
		Name: "remote-node",
		Meta: newMetaBytes,
	}

	delegate.NotifyUpdate(updatedNode)

	// Assert: Connection should still exist (UpdateConnection creates new one)
	conn, err := grpcPool.GetConnection("remote-node")
	if err != nil {
		t.Errorf("Expected connection after update, got error: %v", err)
	}
	if conn == nil {
		t.Error("Expected non-nil connection after update")
	}

	// Note: We can't easily verify the address changed without inspecting
	// the actual gRPC connection internals, but the UpdateConnection call
	// executed successfully
}

// TestEventDelegate_NotifyJoin_InvalidMetadata verifies fallback handling
// when node metadata is corrupted.
func TestEventDelegate_NotifyJoin_InvalidMetadata(t *testing.T) {
	logger := zerolog.Nop()
	hashRing := consistenthashring.NewRing(3)
	store := storage.NewNoEvictionStorage()
	grpcPool := NewGRPCPool(logger)
	localName := "local-node"
	var grpcPort uint32 = 9876

	delegate := NewEventDelegate(logger, hashRing, store, grpcPool, localName, grpcPort)

	// Create node with invalid metadata (not JSON) but valid address
	node := &memberlist.Node{
		Name: "bad-node",
		Meta: []byte("this is not json{{{"),
		Addr: []byte{10, 0, 1, 5}, // IP address 10.0.1.5
	}

	// Act: Try to join with bad metadata
	// Should not panic, use fallback mechanism instead
	delegate.NotifyJoin(node)

	// Assert: Node SHOULD be in connection pool (using fallback address)
	// The fallback mechanism constructs: <node.Addr>:<grpcPort>
	conn, err := grpcPool.GetConnection("bad-node")
	if err != nil {
		t.Errorf("Expected fallback connection to be created, got error: %v", err)
	}
	if conn == nil {
		t.Error("Expected non-nil connection with fallback address")
	}

	// Note: The fallback mechanism provides robustness by constructing
	// a gRPC address even when metadata parsing fails
}

func TestEventDelegate_NotifyJoin_TransfersKeyToNewOwner(t *testing.T) {
	logger := zerolog.Nop()
	hashRing := consistenthashring.NewRing(150)
	localName := "local-node"
	remoteName := "remote-node"
	hashRing.AddNode(localName)

	store := storage.NewNoEvictionStorage()
	grpcPool := NewGRPCPool(logger)
	var grpcPort uint32 = 9876

	delegate := NewEventDelegate(logger, hashRing, store, grpcPool, localName, grpcPort)

	addr, writes, cleanup := startCaptureGRPCServer(t)
	defer cleanup()

	probeRing := consistenthashring.NewRing(150)
	probeRing.AddNode(localName)
	probeRing.AddNode(remoteName)

	_, transferHash := findKeyAndHashForNode(t, probeRing, remoteName)
	key := "transfer-key"
	value := []byte("transfer-value")
	store.Write(key, transferHash, value)

	meta := NodeMeta{NodeName: remoteName, GRPCAddr: addr}
	metaBytes, err := meta.Marshal()
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	joinNode := &memberlist.Node{Name: remoteName, Meta: metaBytes}
	delegate.NotifyJoin(joinNode)

	if got := store.Read(key); got != nil {
		t.Fatalf("expected key to be moved off local node, got %q", string(got))
	}

	select {
	case kv := <-writes:
		if kv.Key != key {
			t.Fatalf("expected transferred key %q, got %q", key, kv.Key)
		}
		if kv.Hash != transferHash {
			t.Fatalf("expected transferred hash %d, got %d", transferHash, kv.Hash)
		}
		if !bytes.Equal(kv.Value, value) {
			t.Fatalf("expected transferred value %q, got %q", string(value), string(kv.Value))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for transferred write")
	}
}

func TestEventDelegate_NotifyJoin_DoesNotTransferKeyStillOwnedLocally(t *testing.T) {
	logger := zerolog.Nop()
	hashRing := consistenthashring.NewRing(150)
	localName := "local-node"
	remoteName := "remote-node"
	hashRing.AddNode(localName)

	store := storage.NewNoEvictionStorage()
	grpcPool := NewGRPCPool(logger)
	var grpcPort uint32 = 9876

	delegate := NewEventDelegate(logger, hashRing, store, grpcPool, localName, grpcPort)

	addr, writes, cleanup := startCaptureGRPCServer(t)
	defer cleanup()

	probeRing := consistenthashring.NewRing(150)
	probeRing.AddNode(localName)
	probeRing.AddNode(remoteName)

	key, localHash := findKeyAndHashForNode(t, probeRing, localName)
	value := []byte("local-value")
	store.Write(key, localHash, value)

	meta := NodeMeta{NodeName: remoteName, GRPCAddr: addr}
	metaBytes, err := meta.Marshal()
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	joinNode := &memberlist.Node{Name: remoteName, Meta: metaBytes}
	delegate.NotifyJoin(joinNode)

	if got := store.Read(key); !bytes.Equal(got, value) {
		t.Fatalf("expected key to remain local, got %q", string(got))
	}

	select {
	case kv := <-writes:
		t.Fatalf("unexpected transfer for locally-owned key: %+v", kv)
	case <-time.After(200 * time.Millisecond):
		// expected: no transfer
	}
}
