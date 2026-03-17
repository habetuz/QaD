package httpserver

import (
	"encoding/json"
	"net/http"
)

type ClusterInfo interface {
	GetNodes() []string

	GetLocalNode() string
}

// HealthHandler handles /health endpoint for liveness checks.
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// ClusterStatus represents the current state of the cluster.
type ClusterStatus struct {
	LocalNode  string   `json:"local_node"`  // This node's address
	TotalNodes int      `json:"total_nodes"` // Number of nodes in cluster
	Nodes      []string `json:"nodes"`       // List of all node addresses
}

// ClusterHandler handles /cluster endpoint for cluster information.
func (s *Server) ClusterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.hashRing == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ClusterStatus{
			LocalNode:  s.selfAddr,
			TotalNodes: 1,
			Nodes:      []string{s.selfAddr},
		})
		return
	}
	clusterInfo, ok := s.hashRing.(ClusterInfo)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "cluster_info_unavailable",
			"node":   s.selfAddr,
		})
		return
	}

	nodes := clusterInfo.GetNodes()

	status := ClusterStatus{
		LocalNode:  clusterInfo.GetLocalNode(),
		TotalNodes: len(nodes),
		Nodes:      nodes,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}
