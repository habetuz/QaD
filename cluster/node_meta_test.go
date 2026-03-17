package cluster

import (
	"strings"
	"testing"
)

// TestNodeMeta_Marshal verifies that NodeMeta can be converted to JSON bytes.
func TestNodeMeta_Marshal(t *testing.T) {
	// Arrange: Create test data
	meta := NodeMeta{
		NodeName: "test-node-123",
		GRPCAddr: "10.0.1.5:9876",
	}

	// Act: Perform the operation we're testing
	data, err := meta.Marshal()

	// Assert: Verify the results
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}

	// Verify we got some data back
	if len(data) == 0 {
		t.Error("Marshal() returned empty data")
	}

	// Verify the data is valid JSON by checking it contains our fields
	jsonStr := string(data)
	if !strings.Contains(jsonStr, "test-node-123") {
		t.Errorf("JSON missing NodeName: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, "10.0.1.5:9876") {
		t.Errorf("JSON missing GRPCAddr: %s", jsonStr)
	}
}

// TestNodeMeta_Marshal_SizeLimit verifies that Marshal enforces the 512-byte limit.
// This is critical because memberlist will reject oversized metadata.
func TestNodeMeta_Marshal_SizeLimit(t *testing.T) {
	// Create metadata with an extremely long node name that will exceed 512 bytes
	// 600 'x' characters will definitely push us over the limit
	meta := NodeMeta{
		NodeName: strings.Repeat("x", 600),
		GRPCAddr: "10.0.1.5:9876",
	}

	// Act: Try to marshal
	_, err := meta.Marshal()

	// Assert: Should fail with size error
	if err == nil {
		t.Fatal("Marshal() should have failed for oversized metadata")
	}

	// Verify the error message mentions the size problem
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected 'too large' error, got: %v", err)
	}
}

// TestNodeMeta_RoundTrip verifies that Marshal → Unmarshal preserves data.
// This is called "round-trip testing" - data should survive a full cycle.
func TestNodeMeta_RoundTrip(t *testing.T) {
	// Arrange: Create original metadata
	original := NodeMeta{
		NodeName: "node-abc-123",
		GRPCAddr: "192.168.1.10:8080",
	}

	// Act: Marshal to bytes
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}

	// Act: Unmarshal back to struct
	var decoded NodeMeta
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}

	// Assert: Verify all fields match
	if decoded.NodeName != original.NodeName {
		t.Errorf("NodeName mismatch: got %q, want %q", decoded.NodeName, original.NodeName)
	}
	if decoded.GRPCAddr != original.GRPCAddr {
		t.Errorf("GRPCAddr mismatch: got %q, want %q", decoded.GRPCAddr, original.GRPCAddr)
	}
}

// TestNodeMeta_Unmarshal_InvalidJSON tests error handling for bad input.
// Always test the unhappy path - what happens when things go wrong?
func TestNodeMeta_Unmarshal_InvalidJSON(t *testing.T) {
	var meta NodeMeta

	// Try to unmarshal invalid JSON
	err := meta.Unmarshal([]byte("not valid json{{{"))

	// Should fail with an error
	if err == nil {
		t.Fatal("Unmarshal() should fail for invalid JSON")
	}

	// Error message should mention unmarshaling
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected error to mention 'unmarshal', got: %v", err)
	}
}

// TestNodeMeta_Unmarshal_MissingFields verifies behavior with incomplete JSON.
func TestNodeMeta_Unmarshal_MissingFields(t *testing.T) {
	// JSON with only one field
	data := []byte(`{"node_name":"test-node"}`)

	var meta NodeMeta
	err := meta.Unmarshal(data)

	// Should succeed - JSON allows missing fields (they default to zero values)
	if err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}

	// Verify the present field was parsed
	if meta.NodeName != "test-node" {
		t.Errorf("NodeName = %q, want %q", meta.NodeName, "test-node")
	}

	// Missing field should be empty string (zero value for string)
	if meta.GRPCAddr != "" {
		t.Errorf("GRPCAddr should be empty, got %q", meta.GRPCAddr)
	}
}

// TestNodeMeta_Marshal_EmptyFields verifies behavior with empty metadata.
func TestNodeMeta_Marshal_EmptyFields(t *testing.T) {
	// Create metadata with empty fields
	meta := NodeMeta{
		NodeName: "",
		GRPCAddr: "",
	}

	// Should still marshal successfully (even if not useful)
	data, err := meta.Marshal()
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}

	// Verify we get valid (though minimal) JSON
	jsonStr := string(data)
	if !strings.Contains(jsonStr, "node_name") {
		t.Errorf("JSON missing node_name field: %s", jsonStr)
	}
}
