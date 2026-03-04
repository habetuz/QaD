package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/habetuz/qad/config"
	grpcserver "github.com/habetuz/qad/grpc_server"
	httpserver "github.com/habetuz/qad/http_server"
	"github.com/habetuz/qad/proto_gen"
	"github.com/habetuz/qad/storage"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

func newGRPCServer() *grpc.Server {
	srv := grpc.NewServer()
	proto_gen.RegisterCommunicationServer(srv, grpcserver.NewServer())
	return srv
}

func newHTTPServer(store storage.Storage) *http.Server {
	return &http.Server{
		Addr:    ":8080",
		Handler: httpserver.NewServer(store),
	}
}

func main() {
	log.Logger = log.Output(
		zerolog.ConsoleWriter{Out: os.Stdout, NoColor: false},
	)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	zerolog.SetGlobalLevel(cfg.LogLevel)

	var store storage.Storage

	switch cfg.EvictionAlgorithm {
	case config.FIFO:
		store = storage.NewFIFOStorage(cfg.StorageSize)
	case config.LRU:
		store = storage.NewLRUStorage(cfg.StorageSize)
	case config.NONE:
		store = storage.NewNoEvictionStorage()
	}

	httpSrv := newHTTPServer(store)
	go func() {
		log.Info().Int("port", int(cfg.HTTPPort)).Msg("Starting HTTP server")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Int("port", int(cfg.HTTPPort)).Msg("Failed to start HTTP server")
		}
	}()

	go func() {
		lc := net.ListenConfig{}
		lis, err := lc.Listen(
			context.Background(),
			"tcp",
			fmt.Sprintf(":%d", cfg.GRPCPort),
		)
		if err != nil {
			log.Fatal().
				Err(err).
				Int("port", int(cfg.GRPCPort)).
				Msg("Failed to create gRPC listener")
		}

		srv := newGRPCServer()

		log.Info().
			Int("port", int(cfg.GRPCPort)).
			Msg("Starting gRPC server")
		if err := srv.Serve(lis); err != nil {
			log.Fatal().Err(err).Msgf("Failed to start gRPC server on :%d", cfg.GRPCPort)
		}
	}()

	select {}
}
