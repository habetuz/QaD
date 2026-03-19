package httpserver

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/habetuz/qad/proto_gen"
	"github.com/habetuz/qad/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockHashRing is a test implementation of HashRing.
type mockHashRing struct {
	nodeAddr string // Address to return for all keys
}

func testHash(key string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return h.Sum64()
}

// GetNodes implements [HashRing].
func (m *mockHashRing) GetNodes() []string {
	return []string{m.nodeAddr}
}

func (m *mockHashRing) NodeOf(key string) (uint64, string) {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return h.Sum64(), m.nodeAddr
}

// mockGRPCPool is a test implementation of GRPCPool.
type mockGRPCPool struct {
	clients map[string]proto_gen.CommunicationClient
}

func newMockGRPCPool() *mockGRPCPool {
	return &mockGRPCPool{
		clients: make(map[string]proto_gen.CommunicationClient),
	}
}

func (m *mockGRPCPool) GetClient(addr string) (proto_gen.CommunicationClient, error) {
	client, ok := m.clients[addr]
	if !ok {
		return nil, fmt.Errorf("no client for address: %s", addr)
	}
	return client, nil
}

func (m *mockGRPCPool) setClient(addr string, client proto_gen.CommunicationClient) {
	m.clients[addr] = client
}

// mockGRPCClient is a test implementation of CommunicationClient.
type mockGRPCClient struct {
	readFunc  func(ctx context.Context, key *proto_gen.Key) (*proto_gen.Value, error)
	writeFunc func(ctx context.Context, kvPair *proto_gen.KeyValuePair) (*proto_gen.Void, error)
}

func (m *mockGRPCClient) Read(ctx context.Context, key *proto_gen.Key, opts ...grpc.CallOption) (*proto_gen.Value, error) {
	if m.readFunc != nil {
		return m.readFunc(ctx, key)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockGRPCClient) Write(ctx context.Context, kvPair *proto_gen.KeyValuePair, opts ...grpc.CallOption) (*proto_gen.Void, error) {
	if m.writeFunc != nil {
		return m.writeFunc(ctx, kvPair)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// TestHandleGet_Local verifies GET requests for keys owned by this node.
func TestHandleGet_Local(t *testing.T) {
	// Setup
	store := storage.NewNoEvictionStorage()
	store.Write("test-key", testHash("test-key"), []byte("test-value"))

	hashRing := &mockHashRing{nodeAddr: "localhost:8080"}
	server := NewServer(store, hashRing, nil, "localhost:8080")

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/test-key", nil)
	w := httptest.NewRecorder()

	// Execute
	server.ServeHTTP(w, req)

	// Verify
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "test-value" {
		t.Errorf("Expected body 'test-value', got '%s'", body)
	}
}

// TestHandleGet_Remote verifies GET requests for keys owned by another node.
func TestHandleGet_Remote(t *testing.T) {
	// Setup local storage (empty)
	store := storage.NewNoEvictionStorage()

	// Setup hash ring to point to remote node
	hashRing := &mockHashRing{nodeAddr: "remote:8080"}

	// Setup mock gRPC client
	pool := newMockGRPCPool()
	mockClient := &mockGRPCClient{
		readFunc: func(ctx context.Context, key *proto_gen.Key) (*proto_gen.Value, error) {
			if key.Key == "remote-key" {
				return &proto_gen.Value{
					Payload: [][]byte{[]byte("remote-value")},
				}, nil
			}
			return nil, status.Error(codes.NotFound, "not found")
		},
	}
	pool.setClient("remote:8080", mockClient)

	server := NewServer(store, hashRing, pool, "localhost:8080")

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/remote-key", nil)
	w := httptest.NewRecorder()

	// Execute
	server.ServeHTTP(w, req)

	// Verify
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "remote-value" {
		t.Errorf("Expected body 'remote-value', got '%s'", body)
	}
}

// TestHandleGet_NotFound verifies 404 response for non-existent keys.
func TestHandleGet_NotFound(t *testing.T) {
	store := storage.NewNoEvictionStorage()
	hashRing := &mockHashRing{nodeAddr: "localhost:8080"}
	server := NewServer(store, hashRing, nil, "localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

// TestHandleGet_RemoteNotFound verifies 404 when remote node doesn't have the key.
func TestHandleGet_RemoteNotFound(t *testing.T) {
	store := storage.NewNoEvictionStorage()
	hashRing := &mockHashRing{nodeAddr: "remote:8080"}
	pool := newMockGRPCPool()

	mockClient := &mockGRPCClient{
		readFunc: func(ctx context.Context, key *proto_gen.Key) (*proto_gen.Value, error) {
			return nil, status.Error(codes.NotFound, "not found")
		},
	}
	pool.setClient("remote:8080", mockClient)

	server := NewServer(store, hashRing, pool, "localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

// TestHandlePost_Local verifies POST requests for keys owned by this node.
func TestHandlePost_Local(t *testing.T) {
	store := storage.NewNoEvictionStorage()
	hashRing := &mockHashRing{nodeAddr: "localhost:8080"}
	server := NewServer(store, hashRing, nil, "localhost:8080")

	req := httptest.NewRequest(http.MethodPost, "/api/test-key", bytes.NewReader([]byte("test-value")))
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", resp.StatusCode)
	}

	// Wait for async write
	time.Sleep(10 * time.Millisecond)

	// Verify value was written
	value := store.Read("test-key")
	if string(value) != "test-value" {
		t.Errorf("Expected stored value 'test-value', got '%s'", value)
	}
}

// TestHandlePost_Remote verifies POST requests forwarded to remote nodes.
func TestHandlePost_Remote(t *testing.T) {
	store := storage.NewNoEvictionStorage()
	hashRing := &mockHashRing{nodeAddr: "remote:8080"}
	pool := newMockGRPCPool()

	// Track if Write was called
	writeCalled := false
	mockClient := &mockGRPCClient{
		writeFunc: func(ctx context.Context, kvPair *proto_gen.KeyValuePair) (*proto_gen.Void, error) {
			writeCalled = true
			if kvPair.Key != "remote-key" {
				t.Errorf("Expected key 'remote-key', got '%s'", kvPair.Key)
			}

			if string(kvPair.Value) != "remote-value" {
				t.Errorf("Expected value 'remote-value', got '%s'", kvPair.Value)
			}
			return &proto_gen.Void{}, nil
		},
	}
	pool.setClient("remote:8080", mockClient)

	server := NewServer(store, hashRing, pool, "localhost:8080")

	req := httptest.NewRequest(http.MethodPost, "/api/remote-key", bytes.NewReader([]byte("remote-value")))
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", resp.StatusCode)
	}

	// Wait for async write
	time.Sleep(10 * time.Millisecond)

	if !writeCalled {
		t.Error("Expected Write to be called on remote client")
	}
}

// TestHandleDelete_Local verifies DELETE requests for local keys.
func TestHandleDelete_Local(t *testing.T) {
	store := storage.NewNoEvictionStorage()
	store.Write("test-key", testHash("test-key"), []byte("test-value"))

	hashRing := &mockHashRing{nodeAddr: "localhost:8080"}
	server := NewServer(store, hashRing, nil, "localhost:8080")

	req := httptest.NewRequest(http.MethodDelete, "/api/test-key", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", resp.StatusCode)
	}

	// Wait for async delete
	time.Sleep(10 * time.Millisecond)

	// Verify key was deleted
	value := store.Read("test-key")
	if value != nil {
		t.Errorf("Expected key to be deleted, but still exists with value '%s'", value)
	}
}

// TestHandleDelete_Remote verifies DELETE returns 501 for remote keys.
func TestHandleDelete_Remote(t *testing.T) {
	store := storage.NewNoEvictionStorage()
	hashRing := &mockHashRing{nodeAddr: "remote:8080"}
	server := NewServer(store, hashRing, nil, "localhost:8080")

	req := httptest.NewRequest(http.MethodDelete, "/api/remote-key", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("Expected status 501, got %d", resp.StatusCode)
	}
}

// TestShouldRouteLocally_WithHashRing verifies local routing with hash ring.
func TestShouldRouteLocally_NoHashRing(t *testing.T) {
	hashRing := &mockHashRing{nodeAddr: "localhost:8080"}
	server := NewServer(storage.NewNoEvictionStorage(), hashRing, nil, "localhost:8080")

	isLocal, _, target := server.shouldRouteLocally("any-key")
	if !isLocal {
		t.Error("Expected local routing when hash ring resolves to self")
	}
	if target != "localhost:8080" {
		t.Errorf("Expected target 'localhost:8080', got '%s'", target)
	}
}

// TestShouldRouteLocally_LocalKey verifies routing for local keys.
func TestShouldRouteLocally_LocalKey(t *testing.T) {
	hashRing := &mockHashRing{nodeAddr: "localhost:8080"}
	server := NewServer(storage.NewNoEvictionStorage(), hashRing, nil, "localhost:8080")

	isLocal, _, target := server.shouldRouteLocally("test-key")
	if !isLocal {
		t.Error("Expected local routing when target matches self")
	}
	if target != "localhost:8080" {
		t.Errorf("Expected target 'localhost:8080', got '%s'", target)
	}
}

// TestShouldRouteLocally_RemoteKey verifies routing for remote keys.
func TestShouldRouteLocally_RemoteKey(t *testing.T) {
	hashRing := &mockHashRing{nodeAddr: "remote:8080"}
	server := NewServer(storage.NewNoEvictionStorage(), hashRing, nil, "localhost:8080")

	isLocal, _, target := server.shouldRouteLocally("test-key")
	if isLocal {
		t.Error("Expected remote routing when target doesn't match self")
	}
	if target != "remote:8080" {
		t.Errorf("Expected target 'remote:8080', got '%s'", target)
	}
}
