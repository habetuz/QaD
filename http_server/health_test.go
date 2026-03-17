package httpserver

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/habetuz/qad/storage"
)

// TestHealthHandler verifies the /health endpoint.
func TestHealthHandler(t *testing.T) {
	server := NewServer(storage.NewNoEvictionStorage(), nil, nil, "localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.HealthHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", result["status"])
	}
}

// TestHealthHandler_InvalidMethod verifies method validation.
func TestHealthHandler_InvalidMethod(t *testing.T) {
	server := NewServer(storage.NewNoEvictionStorage(), nil, nil, "localhost:8080")

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	server.HealthHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

// mockClusterInfoHashRing implements both HashRing and ClusterInfo.
type mockClusterInfoHashRing struct {
	nodes     []string
	localNode string
}

func (m *mockClusterInfoHashRing) GetNode(key string) string {
	if len(m.nodes) == 0 {
		return ""
	}
	return m.nodes[0]
}

func (m *mockClusterInfoHashRing) GetNodes() []string {
	return m.nodes
}

func (m *mockClusterInfoHashRing) GetLocalNode() string {
	return m.localNode
}

// TestClusterHandler_NoHashRing verifies non-distributed mode.
func TestClusterHandler_NoHashRing(t *testing.T) {
	server := NewServer(storage.NewNoEvictionStorage(), nil, nil, "localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/cluster", nil)
	w := httptest.NewRecorder()

	server.ClusterHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var status ClusterStatus
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if status.LocalNode != "localhost:8080" {
		t.Errorf("Expected local_node 'localhost:8080', got '%s'", status.LocalNode)
	}
	if status.TotalNodes != 1 {
		t.Errorf("Expected total_nodes 1, got %d", status.TotalNodes)
	}
	if len(status.Nodes) != 1 || status.Nodes[0] != "localhost:8080" {
		t.Errorf("Expected nodes ['localhost:8080'], got %v", status.Nodes)
	}
}

// TestClusterHandler_WithClusterInfo verifies cluster status reporting.
func TestClusterHandler_WithClusterInfo(t *testing.T) {
	hashRing := &mockClusterInfoHashRing{
		nodes:     []string{"node1:8080", "node2:8080", "node3:8080"},
		localNode: "node1:8080",
	}

	server := NewServer(storage.NewNoEvictionStorage(), hashRing, nil, "node1:8080")

	req := httptest.NewRequest(http.MethodGet, "/cluster", nil)
	w := httptest.NewRecorder()

	server.ClusterHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var status ClusterStatus
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if status.LocalNode != "node1:8080" {
		t.Errorf("Expected local_node 'node1:8080', got '%s'", status.LocalNode)
	}
	if status.TotalNodes != 3 {
		t.Errorf("Expected total_nodes 3, got %d", status.TotalNodes)
	}
	if len(status.Nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(status.Nodes))
	}
}

// TestClusterHandler_InvalidMethod verifies method validation.
func TestClusterHandler_InvalidMethod(t *testing.T) {
	server := NewServer(storage.NewNoEvictionStorage(), nil, nil, "localhost:8080")

	req := httptest.NewRequest(http.MethodPost, "/cluster", nil)
	w := httptest.NewRecorder()

	server.ClusterHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}
