package main

import (
	"context"
	"net"
	"testing"

	"github.com/habetuz/qad/proto_gen"
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
	srv := newGRPCServer()

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
	if st.Code() != codes.Unimplemented {
		t.Errorf("expected code %v, got %v", codes.Unimplemented, st.Code())
	}
}

func TestWrite(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	_, err := client.Write(context.Background(), &proto_gen.KeyValuePair{
		Key:   &proto_gen.Key{Key: "test-key"},
		Value: &proto_gen.Value{Payload: [][]byte{[]byte("test-value")}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.Unimplemented {
		t.Errorf("expected code %v, got %v", codes.Unimplemented, st.Code())
	}
}
