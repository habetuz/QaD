package grpcserver

import (
	"context"

	"github.com/habetuz/qad/proto_gen"
	"github.com/habetuz/qad/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ proto_gen.CommunicationServer = (*Server)(nil)

type Server struct {
	proto_gen.UnimplementedCommunicationServer

	storage storage.Storage
}

// NewServer creates a new server with the specified registry implementation
func NewServer(storage storage.Storage) *Server {
	return &Server{
		storage: storage,
	}
}

func (server *Server) Read(ctx context.Context, key *proto_gen.Key) (*proto_gen.Value, error) {
	if ctx.Err() != nil {
		return nil, status.Error(codes.Canceled, "request cancelled")
	}

	if key == nil || key.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key cannot be empty")
	}

	value := server.storage.Read(key.Key)

	if value == nil {
		return nil, status.Error(codes.NotFound, "key not found in storage")
	}

	return &proto_gen.Value{
		Payload: [][]byte{value},
	}, nil
}

func (server *Server) Write(ctx context.Context, kvPair *proto_gen.KeyValuePair) (*proto_gen.Void, error) {
	if ctx.Err() != nil {
		return nil, status.Error(codes.Canceled, "request cancelled")
	}

	// Validate the key-value pair structure
	if kvPair == nil {
		return nil, status.Error(codes.InvalidArgument, "key-value pair cannot be nil")
	}
	if kvPair.Key == nil || kvPair.Key.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key cannot be empty")
	}
	if kvPair.Value == nil {
		return nil, status.Error(codes.InvalidArgument, "value cannot be nil")
	}

	var fullValue []byte
	for _, chunk := range kvPair.Value.Payload {
		fullValue = append(fullValue, chunk...)
	}

	server.storage.Write(kvPair.Key.Key, fullValue)

	return &proto_gen.Void{}, nil
}
