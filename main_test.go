package main

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

const bufSize = 1024 * 1024

func startTestServer(t *testing.T) (proto_gen.CommunicationClient, func()) {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	srv := newGRPCServer(storage.NewFIFOStorage(100))

	go func() {
		// Serve returns a non-nil error unless Stop or GracefulStop is called.
		// We ignore the error here since stopping the server in cleanup is expected.
		_ = srv.Serve(lis)
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create gRPC client: %v", err)
	}

	client := proto_gen.NewCommunicationClient(conn)

	cleanup := func() {
		conn.Close()
		srv.GracefulStop()
	}

	return client, cleanup
}

func TestRead(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	_, err := client.Read(context.Background(), &proto_gen.Key{Key: "test-key"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected code %v, got %v", codes.NotFound, st.Code())
	}
}

func TestWrite(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	// Write a value
	_, err := client.Write(context.Background(), &proto_gen.KeyValuePair{
		Key:   "test-key",
		Hash:  1234,
		Value: []byte("test-value"),
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify the value was written by reading it back
	resp, err := client.Read(context.Background(), &proto_gen.Key{Key: "test-key"})
	if err != nil {
		t.Fatalf("failed to read written value: %v", err)
	}

	// Reconstruct the value from payload chunks
	var value []byte
	for _, chunk := range resp.Payload {
		value = append(value, chunk...)
	}

	if string(value) != "test-value" {
		t.Errorf("expected value 'test-value', got '%s'", value)
	}
}
