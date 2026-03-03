package grpcserver

import (
	"context"

	"github.com/habetuz/qad/proto_gen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ proto_gen.CommunicationServer = (*Server)(nil)

type Server struct {
	proto_gen.UnimplementedCommunicationServer
}

// NewServer creates a new server with the specified registry implementation
func NewServer() *Server {
	return &Server{}
}

func (server *Server) Read(ctx context.Context, key *proto_gen.Key) (*proto_gen.Value, error) {
	return nil, status.Error(codes.Unimplemented, "Not implemented")
}

func (server *Server) Write(context.Context, *proto_gen.KeyValuePair) (*proto_gen.Void, error) {
	return nil, status.Error(codes.Unimplemented, "Not implemented")
}
