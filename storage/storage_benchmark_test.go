package storage

import (
	"fmt"
	"hash/fnv"
	"os"
	"testing"

	"github.com/rs/zerolog"
)

func TestMain(m *testing.M) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Exit(m.Run())
}

var benchValue = []byte("benchmark-value-sixteen-bytes!!!")

func benchHash(key string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return h.Sum64()
}

// --- NoEvictionStorage ---

func BenchmarkNoEvictionStorage_Write(b *testing.B) {
	s := NewNoEvictionStorage()
	for i := b.N; b.Loop(); i++ {
		key := fmt.Sprintf("key-%d", i)
		s.Write(key, benchHash(key), benchValue)
	}
}

func BenchmarkNoEvictionStorage_Read(b *testing.B) {
	s := NewNoEvictionStorage()
	for i := range 1000 {
		key := fmt.Sprintf("key-%d", i)
		s.Write(key, benchHash(key), benchValue)
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
		key := fmt.Sprintf("key-%d", i)
		s.Write(key, benchHash(key), benchValue)
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
		key := fmt.Sprintf("key-%d", i)
		s.Write(key, benchHash(key), benchValue)
	}
}

func BenchmarkFIFOStorage_WriteWithEviction(b *testing.B) {
	// Small cap forces constant eviction.
	s := NewFIFOStorage(10 * len(benchValue))
	for i := b.N; b.Loop(); i++ {
		key := fmt.Sprintf("key-%d", i)
		s.Write(key, benchHash(key), benchValue)
	}
}

func BenchmarkFIFOStorage_Read(b *testing.B) {
	s := NewFIFOStorage(1 << 30)
	for i := range 1000 {
		key := fmt.Sprintf("key-%d", i)
		s.Write(key, benchHash(key), benchValue)
	}
	b.ResetTimer()
	for i := b.N; b.Loop(); i++ {
		s.Read(fmt.Sprintf("key-%d", i%1000))
	}
}

func BenchmarkFIFOStorage_Delete(b *testing.B) {
	s := NewFIFOStorage(1 << 30)
	for i := range b.N + 1 {
		key := fmt.Sprintf("key-%d", i)
		s.Write(key, benchHash(key), benchValue)
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
		key := fmt.Sprintf("key-%d", i)
		s.Write(key, benchHash(key), benchValue)
	}
}

func BenchmarkLRUStorage_WriteWithEviction(b *testing.B) {
	// Small cap forces constant eviction.
	s := NewLRUStorage(10 * len(benchValue))
	for i := b.N; b.Loop(); i++ {
		key := fmt.Sprintf("key-%d", i)
		s.Write(key, benchHash(key), benchValue)
	}
}

func BenchmarkLRUStorage_Read(b *testing.B) {
	s := NewLRUStorage(1 << 30)
	for i := range 1000 {
		key := fmt.Sprintf("key-%d", i)
		s.Write(key, benchHash(key), benchValue)
	}
	b.ResetTimer()
	for i := b.N; b.Loop(); i++ {
		s.Read(fmt.Sprintf("key-%d", i%1000))
	}
}

func BenchmarkLRUStorage_Delete(b *testing.B) {
	s := NewLRUStorage(1 << 30)
	for i := range b.N + 1 {
		key := fmt.Sprintf("key-%d", i)
		s.Write(key, benchHash(key), benchValue)
	}
	b.ResetTimer()
	for i := b.N; b.Loop(); i++ {
		s.Delete(fmt.Sprintf("key-%d", i))
	}
}
