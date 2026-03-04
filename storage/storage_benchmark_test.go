package storage

import (
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"
)

func TestMain(m *testing.M) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Exit(m.Run())
}

var benchValue = []byte("benchmark-value-sixteen-bytes!!!")

// --- NoEvictionStorage ---

func BenchmarkNoEvictionStorage_Write(b *testing.B) {
	s := NewNoEvictionStorage()
	for i := b.N; b.Loop(); i++ {
		s.Write(fmt.Sprintf("key-%d", i), benchValue)
	}
}

func BenchmarkNoEvictionStorage_Read(b *testing.B) {
	s := NewNoEvictionStorage()
	for i := range 1000 {
		s.Write(fmt.Sprintf("key-%d", i), benchValue)
	}
	b.ResetTimer()
	for i := b.N; b.Loop(); i++ {
		s.Read(fmt.Sprintf("key-%d", i%1000))
	}
}

func BenchmarkNoEvictionStorage_ReadMissing(b *testing.B) {
	s := NewNoEvictionStorage()
	for b.Loop() {
		s.Read("nonexistent")
	}
}

func BenchmarkNoEvictionStorage_Delete(b *testing.B) {
	s := NewNoEvictionStorage()
	for i := range b.N + 1 {
		s.Write(fmt.Sprintf("key-%d", i), benchValue)
	}
	b.ResetTimer()
	for i := b.N; b.Loop(); i++ {
		s.Delete(fmt.Sprintf("key-%d", i))
	}
}

// --- FIFOStorage ---

func BenchmarkFIFOStorage_Write(b *testing.B) {
	s := NewFIFOStorage(1 << 30) // 1 GiB – no eviction pressure
	for i := b.N; b.Loop(); i++ {
		s.Write(fmt.Sprintf("key-%d", i), benchValue)
	}
}

func BenchmarkFIFOStorage_WriteWithEviction(b *testing.B) {
	// Small cap forces constant eviction.
	s := NewFIFOStorage(10 * len(benchValue))
	for i := b.N; b.Loop(); i++ {
		s.Write(fmt.Sprintf("key-%d", i), benchValue)
	}
}

func BenchmarkFIFOStorage_Read(b *testing.B) {
	s := NewFIFOStorage(1 << 30)
	for i := range 1000 {
		s.Write(fmt.Sprintf("key-%d", i), benchValue)
	}
	b.ResetTimer()
	for i := b.N; b.Loop(); i++ {
		s.Read(fmt.Sprintf("key-%d", i%1000))
	}
}

func BenchmarkFIFOStorage_Delete(b *testing.B) {
	s := NewFIFOStorage(1 << 30)
	for i := range b.N + 1 {
		s.Write(fmt.Sprintf("key-%d", i), benchValue)
	}
	b.ResetTimer()
	for i := b.N; b.Loop(); i++ {
		s.Delete(fmt.Sprintf("key-%d", i))
	}
}

// --- LRUStorage ---

func BenchmarkLRUStorage_Write(b *testing.B) {
	s := NewLRUStorage(1 << 30) // 1 GiB – no eviction pressure
	for i := b.N; b.Loop(); i++ {
		s.Write(fmt.Sprintf("key-%d", i), benchValue)
	}
}

func BenchmarkLRUStorage_WriteWithEviction(b *testing.B) {
	// Small cap forces constant eviction.
	s := NewLRUStorage(10 * len(benchValue))
	for i := b.N; b.Loop(); i++ {
		s.Write(fmt.Sprintf("key-%d", i), benchValue)
	}
}

func BenchmarkLRUStorage_Read(b *testing.B) {
	s := NewLRUStorage(1 << 30)
	for i := range 1000 {
		s.Write(fmt.Sprintf("key-%d", i), benchValue)
	}
	b.ResetTimer()
	for i := b.N; b.Loop(); i++ {
		s.Read(fmt.Sprintf("key-%d", i%1000))
	}
}

func BenchmarkLRUStorage_Delete(b *testing.B) {
	s := NewLRUStorage(1 << 30)
	for i := range b.N + 1 {
		s.Write(fmt.Sprintf("key-%d", i), benchValue)
	}
	b.ResetTimer()
	for i := b.N; b.Loop(); i++ {
		s.Delete(fmt.Sprintf("key-%d", i))
	}
}
