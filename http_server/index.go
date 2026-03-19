package httpserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/habetuz/qad/proto_gen"
	"github.com/habetuz/qad/storage"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HashRing interface {
	GetNode(key string) string
	GetNodes() []string
}

type GRPCPool interface {
	GetClient(addr string) (proto_gen.CommunicationClient, error)
}

type Server struct {
	storage  storage.Storage
	hashRing HashRing
	grpcPool GRPCPool
	selfAddr string
}

func NewServer(storage storage.Storage, hashRing HashRing, grpcPool GRPCPool, selfAddr string) *Server {
	return &Server{
		storage:  storage,
		hashRing: hashRing,
		grpcPool: grpcPool,
		selfAddr: selfAddr,
	}
}

// ServeHTTP implements the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[1:]
	path = strings.Trim(path, "/")

	switch path {
	case "health":
		s.HealthHandler(w, r)
		return
	case "cluster":
		s.ClusterHandler(w, r)
		return
	}

	if !strings.HasPrefix(path, "api/") {
		http.Error(w, "Endpoint does not exist", http.StatusNotFound)
	}

	key := strings.TrimPrefix(path, "api/")

	switch r.Method {
	case http.MethodGet:
		s.handleGet(w, r, key)
	case http.MethodPost:
		s.handlePost(w, r, key)
	case http.MethodDelete:
		s.handleDelete(w, r, key)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// shouldRouteLocally determines if a request should be handled locally or forwarded.
func (s *Server) shouldRouteLocally(key string) (bool, string) {
	targetNode := s.hashRing.GetNode(key)

	// Debug logging to trace routing decisions
	isLocal := targetNode == s.selfAddr

	log.Debug().
		Str("key", key).
		Str("target_node", targetNode).
		Str("self_addr", s.selfAddr).
		Bool("is_local", isLocal).
		Msg("Routing decision")

	if isLocal {
		return true, targetNode
	}

	return false, targetNode
}

// handleGet returns the value for the given key, or 404 if not found.
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, key string) {

	if key == "" {
		http.Error(w, "Key cannot be empty", http.StatusBadRequest)
		return
	}

	isLocal, targetNode := s.shouldRouteLocally(key)

	var value []byte
	var err error

	if isLocal {
		value = s.storage.Read(key)
		if value == nil {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}
	} else {
		value, err = s.forwardGet(r.Context(), targetNode, key)
		if err != nil {
			s.handleForwardError(w, err, "GET")
			return
		}
		if value == nil {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(value)))
	w.Write(value)
}

// forwardGet sends a Read request to a remote node via gRPC.
func (s *Server) forwardGet(ctx context.Context, nodeName, key string) ([]byte, error) {
	log.Debug().
		Str("key", key).
		Str("target_node", nodeName).
		Msg("Forwarding GET request")

	client, err := s.grpcPool.GetClient(nodeName)
	if err != nil {
		log.Error().
			Err(err).
			Str("key", key).
			Str("target_node", nodeName).
			Msg("Failed to get gRPC client for forwarding")
		return nil, fmt.Errorf("failed to get client for %s: %w", nodeName, err)
	}

	grpcCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := client.Read(grpcCtx, &proto_gen.Key{Key: key})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			log.Debug().
				Str("key", key).
				Str("target_node", nodeName).
				Msg("Remote node returned NotFound")
			return nil, nil
		}
		log.Warn().
			Err(err).
			Str("key", key).
			Str("target_node", nodeName).
			Msg("gRPC Read failed during forwarding")
		return nil, fmt.Errorf("gRPC Read failed: %w", err)
	}

	var fullValue []byte
	for _, chunk := range resp.Payload {
		fullValue = append(fullValue, chunk...)
	}

	log.Debug().
		Str("key", key).
		Str("target_node", nodeName).
		Int("bytes", len(fullValue)).
		Msg("Successfully forwarded GET request")

	return fullValue, nil
}

// handlePost stores a value, routing to the correct node.
func (s *Server) handlePost(w http.ResponseWriter, r *http.Request, key string) {

	if key == "" {
		http.Error(w, "Key cannot be empty", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	isLocal, targetNode := s.shouldRouteLocally(key)

	if isLocal {
		go s.writeValue(key, body)
	} else {
		// Use background context for async operation since we return immediately
		go s.forwardPost(context.Background(), targetNode, key, body)
	}

	// Return 202 Accepted immediately (async operation)
	w.WriteHeader(http.StatusAccepted)
}

// forwardPost sends a Write request to a remote node via gRPC.
func (s *Server) forwardPost(ctx context.Context, nodeName, key string, value []byte) {
	log.Debug().
		Str("key", key).
		Str("target_node", nodeName).
		Int("value_size", len(value)).
		Msg("Forwarding POST request")

	client, err := s.grpcPool.GetClient(nodeName)
	if err != nil {
		log.Error().
			Err(err).
			Str("key", key).
			Str("target_node", nodeName).
			Msg("Failed to get gRPC client for forwarding write")
		return
	}

	grpcCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = client.Write(grpcCtx, &proto_gen.KeyValuePair{
		Key:   &proto_gen.Key{Key: key},
		Value: &proto_gen.Value{Payload: [][]byte{value}},
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("key", key).
			Str("target_node", nodeName).
			Msg("gRPC Write failed during forwarding")
	}

	log.Debug().
		Str("key", key).
		Str("target_node", nodeName).
		Msg("Successfully forwarded POST request")
}

func (s *Server) writeValue(key string, value []byte) {
	s.storage.Write(key, value)
}

// handleDelete clears the value for the given key.
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, key string) {
	if key == "" {
		http.Error(w, "Key cannot be empty", http.StatusBadRequest)
		return
	}

	isLocal, targetNode := s.shouldRouteLocally(key)

	if isLocal {
		go s.deleteValue(key)
		w.WriteHeader(http.StatusAccepted)
	} else {
		http.Error(w, fmt.Sprintf("Delete not supported for remote key (node: %s)", targetNode), http.StatusNotImplemented)
	}
}

func (s *Server) deleteValue(key string) {
	s.storage.Delete(key)
}

// handleForwardError converts gRPC errors to appropriate HTTP status codes.
func (s *Server) handleForwardError(w http.ResponseWriter, err error, operation string) {
	st, ok := status.FromError(err)
	if !ok {
		http.Error(w, fmt.Sprintf("%s failed: %v", operation, err), http.StatusInternalServerError)
		return
	}

	switch st.Code() {
	case codes.NotFound:
		http.Error(w, "Key not found", http.StatusNotFound)
	case codes.Unavailable:
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
	case codes.DeadlineExceeded:
		http.Error(w, "Request timeout", http.StatusGatewayTimeout)
	default:
		http.Error(w, fmt.Sprintf("%s failed: %s", operation, st.Message()), http.StatusInternalServerError)
	}
}
