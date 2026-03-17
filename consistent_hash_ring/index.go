package consistenthashring

import (
	"fmt"
	"slices"
	"sort"

	"github.com/cespare/xxhash"
)

type virtualNode struct {
	name  string
	index int
	hash  uint64
}

type ConsistentHashRing struct {
	virtualNodes int
	ring         []virtualNode
	nodes        []string
}

func NewRing(virtualNodes int) *ConsistentHashRing {
	return &ConsistentHashRing{
		virtualNodes: virtualNodes,
		ring:         []virtualNode{},
	}
}

func (c *ConsistentHashRing) AddNode(name string) {
	if slices.Contains(c.nodes, name) {
		panic("Duplicate entry into node list")
	}
	c.nodes = append(c.nodes, name)

	for i := range c.virtualNodes {
		node := virtualNode{
			name:  name,
			index: i,
			hash:  xxhash.Sum64String(name + fmt.Sprint(i)),
		}

		c.ring = sortInto(c.ring, node)
	}
}

func (c *ConsistentHashRing) RemoveNode(name string) {
	if !slices.Contains(c.nodes, name) {
		panic("node is not in ring")
	}
	c.nodes = slices.DeleteFunc(c.nodes, func(node string) bool {
		return node == name
	})

	c.ring = slices.DeleteFunc(c.ring, func(vn virtualNode) bool { return vn.name == name })
}

func (c *ConsistentHashRing) NodeOf(key string) string {
	hash := xxhash.Sum64String(key)
	return c.NodeOfDirect(hash)
}

// GetNode is an alias for NodeOf to satisfy the httpserver.HashRing interface.
func (c *ConsistentHashRing) GetNode(key string) string {
	return c.NodeOf(key)
}

func (c *ConsistentHashRing) NodeOfDirect(hash uint64) string {
	// Handle empty ring - return empty string
	if len(c.ring) == 0 {
		return ""
	}

	// Binary search for the rightmost node with hash <= input hash.
	i := sort.Search(len(c.ring), func(i int) bool { return c.ring[i].hash > hash }) - 1
	if i < 0 {
		// No node has a hash <= input: wrap around to the largest.
		i = len(c.ring) - 1
	}
	return c.ring[i].name
}

func sortInto(slice []virtualNode, node virtualNode) []virtualNode {
	// Grow the slice by one to make room for the new element.
	slice = append(slice, virtualNode{})
	// Find the index of the first element whose hash exceeds the new node's hash.
	// Search over len-1 to exclude the zero-value sentinel we just appended.
	i := sort.Search(len(slice)-1, func(i int) bool { return slice[i].hash > node.hash })
	// Shift everything from i onward one position to the right to open the slot.
	copy(slice[i+1:], slice[i:])
	slice[i] = node
	return slice
}
