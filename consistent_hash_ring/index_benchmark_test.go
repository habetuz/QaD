package consistenthashring

import (
	"fmt"
	"testing"
)

func BenchmarkAddNode(b *testing.B) {
	for b.Loop() {
		ring := NewRing(100)
		for i := range 10 {
			ring.AddNode(fmt.Sprintf("node%d", i))
		}
	}
}

func BenchmarkNodeOf(b *testing.B) {
	ring := NewRing(100)
	for i := range 10 {
		ring.AddNode(fmt.Sprintf("node%d", i))
	}

	b.ResetTimer()
	for i := b.N; b.Loop(); i++ {
		ring.NodeOf(fmt.Sprintf("key-%d", i))
	}
}

func BenchmarkNodeOfDirect(b *testing.B) {
	ring := NewRing(100)
	for i := range 10 {
		ring.AddNode(fmt.Sprintf("node%d", i))
	}

	b.ResetTimer()
	for i := uint64(0); b.Loop(); i++ {
		ring.NodeOfDirect(i)
	}
}

func BenchmarkAddNode_VirtualNodes(b *testing.B) {
	for _, vn := range []int{10, 100, 500} {
		b.Run(fmt.Sprintf("vn=%d", vn), func(b *testing.B) {
			for b.Loop() {
				ring := NewRing(vn)
				for i := range 10 {
					ring.AddNode(fmt.Sprintf("node%d", i))
				}
			}
		})
	}
}

func BenchmarkNodeOf_RingSize(b *testing.B) {
	for _, nodes := range []int{3, 10, 50} {
		b.Run(fmt.Sprintf("nodes=%d", nodes), func(b *testing.B) {
			ring := NewRing(100)
			for i := range nodes {
				ring.AddNode(fmt.Sprintf("node%d", i))
			}

			b.ResetTimer()
			for i := b.N; b.Loop(); i++ {
				ring.NodeOf(fmt.Sprintf("key-%d", i))
			}
		})
	}
}
