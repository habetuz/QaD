package storage

import (
	"sync"

	"github.com/rs/zerolog/log"
)

var _ Storage = (*FIFOStorage)(nil)

type FIFOStorage struct {
	mu      sync.Mutex
	storage map[string][]byte
	order   []string // insertion order
	maxSize int
	curSize int
}

func NewFIFOStorage(maxSize int) *FIFOStorage {
	return &FIFOStorage{
		storage: make(map[string][]byte),
		maxSize: maxSize,
	}
}

// Read implements [Storage].
func (s *FIFOStorage) Read(key string) []byte {
	// No locking as concurrent writes are fine here
	value, ok := s.storage[key]
	log.Debug().Str("key", key).Bool("exists", ok).Int("length", len(value)).Msg("FIFO read key")
	if ok {
		return value
	}
	return nil
}

// Write implements [Storage].
func (s *FIFOStorage) Write(key string, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If key already exists, remove its old size contribution.
	if old, ok := s.storage[key]; ok {
		s.curSize -= len(old)
	} else {
		s.order = append(s.order, key)
	}

	s.storage[key] = value
	s.curSize += len(value)

	// Evict oldest entries until we're within the size limit.
	for s.curSize > s.maxSize && len(s.order) > 0 {
		oldest := s.order[0]
		s.order = s.order[1:]
		s.curSize -= len(s.storage[oldest])
		delete(s.storage, oldest)
		log.Debug().Str("key", oldest).Msg("FIFO evicted key")
	}

	log.Debug().Str("key", key).Int("length", len(value)).Msg("FIFO wrote key")
}

// Delete implements [Storage].
func (s *FIFOStorage) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if old, ok := s.storage[key]; ok {
		s.curSize -= len(old)
		delete(s.storage, key)
		for i, k := range s.order {
			if k == key {
				s.order = append(s.order[:i], s.order[i+1:]...)
				break
			}
		}
	}
	log.Debug().Str("key", key).Msg("FIFO deleted key")
}
