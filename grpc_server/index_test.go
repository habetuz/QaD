package grpcserver

import (
	"context"
	"net"
	"testing"

	"github.com/habetuz/qad/proto_gen"
	"github.com/habetuz/qad/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// bufSize is the size of the in-memory buffer for the bufconn listener.
// This simulates a network connection without using actual TCP sockets.
const bufSize = 1024 * 1024

// setupTestServer creates an in-memory gRPC server for testing.
// It uses bufconn to avoid binding to real network ports.
//
// bufconn Pattern:
//   - Creates an in-memory network connection
//   - No ports, no TCP overhead, no interference with other tests
//   - Perfect for unit testing gRPC services
//
// Returns:
//   - *grpc.ClientConn: Client connection to the test server
//   - storage.Storage: The storage backend (for verification)
//   - func(): Cleanup function to close connections
func setupTestServer(t *testing.T) (*grpc.ClientConn, storage.Storage, func()) {
	// Create an in-memory listener with a 1MB buffer
	// This acts like a real network connection but stays in memory
	lis := bufconn.Listen(bufSize)

	// Create the storage backend
	// Using NoEvictionStorage for predictable test behavior
	store := storage.NewNoEvictionStorage()

	// Create the gRPC server instance
	grpcServer := grpc.NewServer()
	
	// Register our service implementation
	proto_gen.RegisterCommunicationServer(grpcServer, NewServer(store))

	// Start the server in a goroutine
	// The server runs until we call Stop() in cleanup
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Errorf("Server exited with error: %v", err)
		}
	}()

	// Create a client connection to our in-memory server
	// The dialer intercepts connection attempts and routes to bufconn
	conn, err := grpc.NewClient(
		"passthrough://bufnet",  // Dummy address (not used)
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()  // Use bufconn instead of real TCP
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	// Return cleanup function to close everything properly
	cleanup := func() {
		conn.Close()
		grpcServer.Stop()
		lis.Close()
	}

	return conn, store, cleanup
}

// TestRead_Success verifies that Read returns the correct value for an existing key.
func TestRead_Success(t *testing.T) {
	conn, store, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a gRPC client
	client := proto_gen.NewCommunicationClient(conn)

	// Pre-populate storage with test data
	testKey := "test-key"
	testValue := []byte("test-value")
	store.Write(testKey, testValue)

	// Execute the Read RPC
	ctx := context.Background()
	resp, err := client.Read(ctx, &proto_gen.Key{Key: testKey})

	// Verify success
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Verify the returned value matches
	if len(resp.Payload) != 1 {
		t.Fatalf("Expected 1 payload chunk, got %d", len(resp.Payload))
	}
	if string(resp.Payload[0]) != string(testValue) {
		t.Errorf("Expected value %q, got %q", testValue, resp.Payload[0])
	}
}

// TestRead_NotFound verifies that Read returns NotFound for non-existent keys.
func TestRead_NotFound(t *testing.T) {
	conn, _, cleanup := setupTestServer(t)
	defer cleanup()

	client := proto_gen.NewCommunicationClient(conn)

	// Try to read a key that doesn't exist
	ctx := context.Background()
	resp, err := client.Read(ctx, &proto_gen.Key{Key: "nonexistent"})

	// Verify we got a NotFound error
	if err == nil {
		t.Fatal("Expected error for non-existent key, got nil")
	}

	// Extract the gRPC status code
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.NotFound {
		t.Errorf("Expected NotFound code, got %v", st.Code())
	}

	if resp != nil {
		t.Errorf("Expected nil response, got %v", resp)
	}
}

// TestRead_EmptyKey verifies that Read rejects empty keys.
func TestRead_EmptyKey(t *testing.T) {
	conn, _, cleanup := setupTestServer(t)
	defer cleanup()

	client := proto_gen.NewCommunicationClient(conn)

	// Try to read with an empty key
	ctx := context.Background()
	_, err := client.Read(ctx, &proto_gen.Key{Key: ""})

	// Verify we got an InvalidArgument error
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument code, got %v", st.Code())
	}
}

// TestWrite_Success verifies that Write stores data correctly.
func TestWrite_Success(t *testing.T) {
	conn, store, cleanup := setupTestServer(t)
	defer cleanup()

	client := proto_gen.NewCommunicationClient(conn)

	// Execute the Write RPC
	testKey := "write-key"
	testValue := []byte("write-value")
	ctx := context.Background()
	
	_, err := client.Write(ctx, &proto_gen.KeyValuePair{
		Key:   &proto_gen.Key{Key: testKey},
		Value: &proto_gen.Value{Payload: [][]byte{testValue}},
	})

	// Verify the write succeeded
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify the data was written to storage
	storedValue := store.Read(testKey)
	if storedValue == nil {
		t.Fatal("Value not found in storage after write")
	}
	if string(storedValue) != string(testValue) {
		t.Errorf("Expected stored value %q, got %q", testValue, storedValue)
	}
}

// TestWrite_MultipleChunks verifies that Write handles chunked data correctly.
func TestWrite_MultipleChunks(t *testing.T) {
	conn, store, cleanup := setupTestServer(t)
	defer cleanup()

	client := proto_gen.NewCommunicationClient(conn)

	// Write data split across multiple chunks
	testKey := "chunked-key"
	chunk1 := []byte("hello-")
	chunk2 := []byte("world")
	ctx := context.Background()
	
	_, err := client.Write(ctx, &proto_gen.KeyValuePair{
		Key: &proto_gen.Key{Key: testKey},
		Value: &proto_gen.Value{
			Payload: [][]byte{chunk1, chunk2},
		},
	})

	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify the chunks were concatenated
	storedValue := store.Read(testKey)
	expected := "hello-world"
	if string(storedValue) != expected {
		t.Errorf("Expected %q, got %q", expected, storedValue)
	}
}

// TestWrite_EmptyKey verifies that Write rejects empty keys.
func TestWrite_EmptyKey(t *testing.T) {
	conn, _, cleanup := setupTestServer(t)
	defer cleanup()

	client := proto_gen.NewCommunicationClient(conn)

	ctx := context.Background()
	_, err := client.Write(ctx, &proto_gen.KeyValuePair{
		Key:   &proto_gen.Key{Key: ""},
		Value: &proto_gen.Value{Payload: [][]byte{[]byte("value")}},
	})

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument code, got %v", st.Code())
	}
}

// TestWrite_NilValue verifies that Write rejects nil values.
func TestWrite_NilValue(t *testing.T) {
	conn, _, cleanup := setupTestServer(t)
	defer cleanup()

	client := proto_gen.NewCommunicationClient(conn)

	ctx := context.Background()
	_, err := client.Write(ctx, &proto_gen.KeyValuePair{
		Key:   &proto_gen.Key{Key: "test-key"},
		Value: nil,
	})

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument code, got %v", st.Code())
	}
}