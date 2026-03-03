package main

import (
	"context"
	"fmt"
	"net"
	"os"

	grpcserver "github.com/habetuz/qad/grpc_server"
	"github.com/habetuz/qad/proto_gen"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

func newGRPCServer() *grpc.Server {
	srv := grpc.NewServer()
	proto_gen.RegisterCommunicationServer(srv, grpcserver.NewServer())
	return srv
}

func main() {
	log.Logger = log.Output(
		zerolog.ConsoleWriter{Out: os.Stdout, NoColor: false},
	)

	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	port := 9876

	lc := net.ListenConfig{}
	lis, err := lc.Listen(
		context.Background(),
		"tcp",
		fmt.Sprintf(":%d", port),
	)
	if err != nil {
		log.Fatal().
			Err(err).
			Int("port", port).
			Msg("Failed to create gRPC listener")
	}

	srv := newGRPCServer()

	log.Info().
		Int("port", port).
		Msg("Setup finished. Starting to listen")
	if err := srv.Serve(lis); err != nil {
		log.Fatal().Err(err).Msgf("Failed to start gRPC server on :%d", port)
	}
}
