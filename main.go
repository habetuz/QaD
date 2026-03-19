package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/habetuz/qad/cluster"
	"github.com/habetuz/qad/config"
	consistenthashring "github.com/habetuz/qad/consistent_hash_ring"
	grpcserver "github.com/habetuz/qad/grpc_server"
	httpserver "github.com/habetuz/qad/http_server"
	"github.com/habetuz/qad/proto_gen"
	"github.com/habetuz/qad/storage"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

func main() {
	// ============================================================
	// PHASE 1: LOGGING SETUP
	// ============================================================
	// Initialize structured logging with console output.
	// This should be first so all subsequent code can log properly.
	log.Logger = log.Output(
		zerolog.ConsoleWriter{Out: os.Stdout, NoColor: false},
	)

	// ============================================================
	// PHASE 2: CONFIGURATION LOADING
	// ============================================================
	// Load configuration from environment variables and .env file.
	// Configuration determines how all other components behave.
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Set the global log level from config
	zerolog.SetGlobalLevel(cfg.LogLevel)

	log.Info().
		Str("log_level", cfg.LogLevel.String()).
		Uint32("http_port", cfg.HTTPPort).
		Uint32("grpc_port", cfg.GRPCPort).
		Int("cluster_port", cfg.ClusterPort).
		Str("node_name", cfg.NodeName).
		Int("storage_size", cfg.StorageSize).
		Str("eviction_algorithm", string(cfg.EvictionAlgorithm)).
		Msg("Configuration loaded")

	// ============================================================
	// PHASE 3: STORAGE INITIALIZATION
	// ============================================================
	// Create the local storage backend based on eviction algorithm.
	// Storage is the foundation - it must exist before servers that use it.
	var store storage.Storage

	switch cfg.EvictionAlgorithm {
	case config.FIFO:
		store = storage.NewFIFOStorage(cfg.StorageSize)
		log.Info().Msg("Using FIFO eviction strategy")
	case config.LRU:
		store = storage.NewLRUStorage(cfg.StorageSize)
		log.Info().Msg("Using LRU eviction strategy")
	case config.NONE:
		store = storage.NewNoEvictionStorage()
		log.Info().Msg("Using no-eviction strategy")
	default:
		log.Fatal().
			Str("algorithm", string(cfg.EvictionAlgorithm)).
			Msg("Unknown eviction algorithm")
	}

	// ============================================================
	// PHASE 4: CLUSTER MEMBERSHIP INITIALIZATION
	// ============================================================
	// Create the memberlist-based cluster manager.
	// The Manager internally creates and manages:
	//   - ConsistentHashRing (for key distribution)
	//   - GRPCPool (for node-to-node connections)
	//   - EventDelegate (for join/leave/update events)
	clusterMgr, err := cluster.NewManager(cfg, log.Logger, store)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create cluster manager")
	}

	log.Info().
		Str("node_name", cfg.NodeName).
		Int("cluster_port", cfg.ClusterPort).
		Msg("Cluster manager initialized")

	// Get references to the components managed by the cluster
	ring := clusterMgr.GetHashRing()
	pool := clusterMgr.GetGRPCPool()

	// ============================================================
	// PHASE 5: JOIN CLUSTER
	// ============================================================
	// If seed nodes are configured, join the existing cluster.
	// Otherwise, this node becomes the first node in a new cluster.
	if err := clusterMgr.Join(); err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to join cluster")
	}

	// ============================================================
	// PHASE 6: gRPC SERVER INITIALIZATION & START
	// ============================================================
	// Create and start the gRPC server for node-to-node communication.
	// Other nodes will call our Read/Write RPCs to access data stored here.
	grpcServer := newGRPCServer(store)

	// Create TCP listener for gRPC
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		log.Fatal().
			Err(err).
			Uint32("port", cfg.GRPCPort).
			Msg("Failed to create gRPC listener")
	}

	// Start gRPC server in background goroutine
	go func() {
		log.Info().
			Uint32("port", cfg.GRPCPort).
			Msg("Starting gRPC server")

		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatal().
				Err(err).
				Uint32("port", cfg.GRPCPort).
				Msg("gRPC server failed")
		}
	}()

	// ============================================================
	// PHASE 7: HTTP SERVER INITIALIZATION & START
	// ============================================================
	// Create and start the HTTP server for client-facing requests.
	// Clients will send GET/POST/DELETE requests here.
	// The server uses the ring to determine which node should handle each key.
	httpServer := newHTTPServer(store, ring, pool, cfg.NodeName, cfg.HTTPPort)

	// Start HTTP server in background goroutine
	go func() {
		log.Info().
			Uint32("port", cfg.HTTPPort).
			Msg("Starting HTTP server")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().
				Err(err).
				Uint32("port", cfg.HTTPPort).
				Msg("HTTP server failed")
		}
	}()

	// ============================================================
	// PHASE 8: GRACEFUL SHUTDOWN HANDLING
	// ============================================================
	// Wait for interrupt signal (Ctrl+C) or termination signal.
	// When received, gracefully shut down all components.

	// Create channel to receive OS signals
	sigChan := make(chan os.Signal, 1)

	// Register to receive SIGINT (Ctrl+C) and SIGTERM (kill)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a signal
	sig := <-sigChan
	log.Info().
		Str("signal", sig.String()).
		Msg("Received shutdown signal, starting graceful shutdown")

	// Create a context with timeout for shutdown operations
	// If shutdown takes longer than 30 seconds, force quit
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown sequence - order matters!

	// 1. Stop accepting new HTTP requests
	log.Info().Msg("Shutting down HTTP server")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error shutting down HTTP server")
	}

	// 2. Stop accepting new gRPC requests
	log.Info().Msg("Shutting down gRPC server")
	// GracefulStop waits for ongoing RPCs to complete
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	// Wait for graceful stop or timeout
	select {
	case <-stopped:
		log.Info().Msg("gRPC server stopped gracefully")
	case <-shutdownCtx.Done():
		log.Warn().Msg("gRPC server shutdown timeout, forcing stop")
		grpcServer.Stop()
	}

	// 3. Leave the cluster so other nodes know we're gone
	log.Info().Msg("Leaving cluster")
	if err := clusterMgr.Leave(); err != nil {
		log.Error().Err(err).Msg("Error leaving cluster")
	}

	// Note: gRPC connection pool is closed by clusterMgr.Leave()

	log.Info().Msg("Shutdown complete")
}

// newGRPCServer creates a gRPC server with the Communication service registered.
// The server handles Read and Write requests from other nodes in the cluster.
func newGRPCServer(store storage.Storage) *grpc.Server {
	// Create gRPC server with default options
	srv := grpc.NewServer()

	// Register our Communication service implementation
	proto_gen.RegisterCommunicationServer(
		srv,
		grpcserver.NewServer(store),
	)

	return srv
}

// newHTTPServer creates an HTTP server for client requests.
// It uses the hash ring to route requests to the correct node,
// and the gRPC pool to forward requests when needed.
func newHTTPServer(
	store storage.Storage,
	ring *consistenthashring.ConsistentHashRing,
	pool *cluster.GRPCPool,
	nodeName string,
	httpPort uint32,
) *http.Server {
	return &http.Server{
		Addr: fmt.Sprintf(":%d", httpPort),
		Handler: httpserver.NewServer(
			store,
			ring,
			pool,
			nodeName,
		),
		// Timeouts prevent slow clients from holding connections indefinitely
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
