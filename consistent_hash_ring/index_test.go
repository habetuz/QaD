package consistenthashring

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"testing"
)

func TestAddNode_RingSize(t *testing.T) {
	t.Parallel()
	ring := NewRing(3)
	ring.AddNode("nodeA")

	if len(ring.ring) != 3 {
		t.Errorf("expected 3 virtual nodes, got %d", len(ring.ring))
	}
}

func TestAddNode_MultipleNodes(t *testing.T) {
	t.Parallel()
	ring := NewRing(3)
	ring.AddNode("nodeA")
	ring.AddNode("nodeB")

	if len(ring.ring) != 6 {
		t.Errorf("expected 6 virtual nodes, got %d", len(ring.ring))
	}
}

func TestAddNode_Duplicate_Panics(t *testing.T) {
	t.Parallel()
	ring := NewRing(3)
	ring.AddNode("nodeA")

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate AddNode, but did not panic")
		}
	}()
	ring.AddNode("nodeA")
}

func TestRingIsSorted(t *testing.T) {
	t.Parallel()
	ring := NewRing(5)
	ring.AddNode("alpha")
	ring.AddNode("beta")
	ring.AddNode("gamma")

	if !sort.SliceIsSorted(ring.ring, func(i, j int) bool {
		return ring.ring[i].hash < ring.ring[j].hash
	}) {
		t.Error("ring is not sorted by hash after adding nodes")
	}
}

func TestNodeOfDirect_WrapAround(t *testing.T) {
	t.Parallel()
	// Hash 0 is smaller than any real xxhash output, so it should wrap
	// around to the node with the largest hash in the ring.
	ring := NewRing(1)
	ring.AddNode("nodeA")
	ring.AddNode("nodeB")

	largest := ring.ring[len(ring.ring)-1]
	got := ring.NodeOfDirect(0)

	if got != largest.name {
		t.Errorf("wrap-around: expected node %q (largest hash), got %q", largest.name, got)
	}
}

func TestNodeOfDirect_MaxHash(t *testing.T) {
	t.Parallel()
	// math.MaxUint64 is >= all hashes, so it should return the node with
	// the largest hash (last in the sorted ring).
	ring := NewRing(1)
	ring.AddNode("nodeA")
	ring.AddNode("nodeB")

	largest := ring.ring[len(ring.ring)-1]
	got := ring.NodeOfDirect(math.MaxUint64)

	if got != largest.name {
		t.Errorf("max hash: expected node %q, got %q", largest.name, got)
	}
}

func TestNodeOfDirect_ClosestLowerHash(t *testing.T) {
	t.Parallel()
	// A hash just above ring[0].hash should map to ring[0].
	ring := NewRing(1)
	ring.AddNode("nodeA")
	ring.AddNode("nodeB")

	target := ring.ring[0]
	// ring[0].hash + 1 is still <= ring[1].hash (assuming no collision), so
	// the closest-lower node is ring[0].
	if target.hash < math.MaxUint64 && (len(ring.ring) < 2 || target.hash+1 <= ring.ring[1].hash) {
		got := ring.NodeOfDirect(target.hash + 1)
		if got != target.name {
			t.Errorf("expected %q for hash just above ring[0], got %q", target.name, got)
		}
	}
}

func TestNodeOf_Consistency(t *testing.T) {
	t.Parallel()
	ring := NewRing(10)
	ring.AddNode("a")
	ring.AddNode("b")
	ring.AddNode("c")

	key := "some-cache-key"
	_, first := ring.NodeOf(key)
	for range 100 {
		if _, got := ring.NodeOf(key); got != first {
			t.Errorf("NodeOf is not consistent: got %q, want %q", got, first)
		}
	}
}

func TestNodeOf_DistributesAcrossNodes(t *testing.T) {
	t.Parallel()
	// With enough virtual nodes all three physical nodes should receive
	// at least one key from a reasonably large key set.
	ring := NewRing(100)
	ring.AddNode("a")
	ring.AddNode("b")
	ring.AddNode("c")

	counts := map[string]int{}
	for i := range 300 {
		_, node := ring.NodeOf(fmt.Sprintf("key-%d", i))
		counts[node]++
	}

	if len(counts) != 3 {
		t.Errorf("expected keys distributed across 3 nodes, got %d node(s): %v", len(counts), counts)
	}
}

func TestSortInto_MaintainsOrder(t *testing.T) {
	t.Parallel()
	var slice []virtualNode

	nodes := []virtualNode{
		{name: "c", hash: 300},
		{name: "a", hash: 100},
		{name: "b", hash: 200},
	}
	for _, n := range nodes {
		slice = sortInto(slice, n)
	}

	for i := 1; i < len(slice); i++ {
		if slice[i-1].hash >= slice[i].hash {
			t.Errorf("ring not sorted at index %d: %d >= %d", i, slice[i-1].hash, slice[i].hash)
		}
	}
	if slice[0].name != "a" || slice[1].name != "b" || slice[2].name != "c" {
		t.Errorf("unexpected order: %v", slice)
	}
}

// verifies that removing a node acutally removes it from the physical nodes list
func TestRemoveNode_UpdateNodesList(t *testing.T) {
	t.Parallel()

	ring := NewRing(3)

	ring.AddNode("nodeA")
	ring.AddNode("nodeB")
	ring.AddNode("nodeC")

	if len(ring.nodes) != 3 {
		t.Fatalf("expected 3 nodes before removal, got %d", len(ring.nodes))
	}
	ring.RemoveNode("nodeB")
	if len(ring.nodes) != 2 {
		t.Fatalf("expected 2 nodes before removal, got %d", len(ring.nodes))
	}

	if slices.Contains(ring.nodes, "nodeB") {
		t.Error("nodeB should have been removed from nodes list")
	}

	if !slices.Contains(ring.nodes, "nodeA") {
		t.Error("nodeA should still be in nodes list")
	}

	if !slices.Contains(ring.nodes, "nodeC") {
		t.Error("nodeC should still be in nodes list")
	}
}

func TestRemoveNode_RemovesVirtualNodes(t *testing.T) {
	t.Parallel()

	virtualNodesCount := 5
	ring := NewRing(virtualNodesCount)

	ring.AddNode("nodeA")
	ring.AddNode("nodeB")
	ring.AddNode("nodeC")

	expectedVirtualNodes := 15
	if len(ring.ring) != expectedVirtualNodes {
		t.Fatalf("expected %d virtual nodes, got %d", expectedVirtualNodes, len(ring.ring))
	}

	ring.RemoveNode("nodeB")

	expectedAfterRemoval := 10
	if len(ring.ring) != expectedAfterRemoval {
		t.Errorf("expected %d virtual nodes after removal, got %d", expectedAfterRemoval, len(ring.ring))
	}

	for i, vn := range ring.ring {
		if vn.name == "nodeB" {
			t.Errorf("found nodeB virtual node at index %d after removal: %+v", i, vn)
		}
	}
}

// TestRemoveNode_NonExistentNode_Panics verifies that trying to remove
// a node that doesn't exist causes a panic.
func TestRemoveNode_NonExistentNode_Panics(t *testing.T) {
	t.Parallel()

	ring := NewRing(3)
	ring.AddNode("nodeA")

	defer func() {
		if r := recover(); r == nil {

			t.Error("expected panic when removing non-existent node, but did not panic")
		}
	}()

	ring.RemoveNode("nodeB")

}

// TestRemoveNode_KeyRedistribution verifies that after removing a node,
// keys that were assigned to that node get reassigned to other nodes.
func TestRemoveNode_KeyRedistribution(t *testing.T) {
	t.Parallel()

	ring := NewRing(10)
	ring.AddNode("nodeA")
	ring.AddNode("nodeB")
	ring.AddNode("nodeC")

	var keysOnB []string
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		if _, node := ring.NodeOf(key); node == "nodeB" {
			keysOnB = append(keysOnB, key)
			if len(keysOnB) >= 10 {
				break
			}
		}
	}

	if len(keysOnB) == 0 {
		t.Fatal("could not find any keys that map to nodeB")
	}

	ring.RemoveNode("nodeB")

	for _, key := range keysOnB {
		_, node := ring.NodeOf(key)
		if node == "nodeB" {
			t.Errorf("key %q still maps to nodeB after removal", key)
		}

		if node != "nodeA" && node != "nodeC" {
			t.Errorf("key %q maps to unexpected node %q after nodeB removal", key, node)
		}
	}
}
