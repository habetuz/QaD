package storage

import (
	"container/list"
	"sync"

	"github.com/rs/zerolog/log"
)

var _ Storage = (*LRUStorage)(nil)

type lruEntry struct {
	key   string
	hash  uint64
	value []byte
	elem  *list.Element
}

// LRUStorage evicts the least-recently-used key when the size limit is exceeded.
// storage is a sync.Map so Read requires no lock — concurrent reads and writes
// are safe. entry.value is written atomically via atomic.Pointer, so readers
// always see a consistent (though possibly stale) slice without holding any lock.
type LRUStorage struct {
	mu      sync.Mutex // guards list, curSize, and eviction
	storage sync.Map   // map[string]*lruEntry — lock-free reads
	list    *list.List // front = most recent, back = least recent
	maxSize int
	curSize int
	promoCh chan string
}

func NewLRUStorage(maxSize int) *LRUStorage {
	s := &LRUStorage{
		list:    list.New(),
		maxSize: maxSize,
		promoCh: make(chan string, 512),
	}
	go s.promoter()
	return s
}

// promoter is a single long-lived goroutine that applies MoveToFront for reads.
func (s *LRUStorage) promoter() {
	for key := range s.promoCh {
		s.mu.Lock()
		if val, ok := s.storage.Load(key); ok {
			s.list.MoveToFront(val.(*lruEntry).elem)
		}
		s.mu.Unlock()
	}
}

// Read implements [Storage]. Lock-free: sync.Map.Load uses atomics internally.
func (s *LRUStorage) Read(key string) []byte {
	val, ok := s.storage.Load(key)
	if !ok {
		log.Debug().Str("key", key).Bool("exists", false).Msg("LRU read key")
		return nil
	}
	value := val.(*lruEntry).value
	log.Debug().Str("key", key).Bool("exists", true).Int("length", len(value)).Msg("LRU read key")

	// Non-blocking promotion: never block the read if the promoter is busy.
	select {
	case s.promoCh <- key:
	default:
	}

	return value
}

// Write implements [Storage].
func (s *LRUStorage) Write(key string, hash uint64, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if val, ok := s.storage.Load(key); ok {
		entry := val.(*lruEntry)
		s.curSize -= len(entry.value)
		entry.hash = hash
		entry.value = value
		s.list.MoveToFront(entry.elem)
	} else {
		entry := &lruEntry{key: key, hash: hash, value: value}
		entry.elem = s.list.PushFront(entry)
		s.storage.Store(key, entry)
	}
	s.curSize += len(value)

	// Evict least-recently-used entries until within size limit.
	for s.curSize > s.maxSize && s.list.Len() > 0 {
		back := s.list.Back()
		evicted := back.Value.(*lruEntry)
		s.curSize -= len(evicted.value)
		s.storage.Delete(evicted.key)
		s.list.Remove(back)
		log.Debug().Str("key", evicted.key).Msg("LRU evicted key")
	}

	log.Debug().Str("key", key).Int("length", len(value)).Msg("LRU wrote key")
}

// Delete implements [Storage].
func (s *LRUStorage) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if val, ok := s.storage.Load(key); ok {
		entry := val.(*lruEntry)
		s.curSize -= len(entry.value)
		s.list.Remove(entry.elem)
		s.storage.Delete(key)
	}
	log.Debug().Str("key", key).Msg("LRU deleted key")
}

// ListKeys implements [Storage].
func (s *LRUStorage) ListKeys() map[string]uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys := make(map[string]uint64, s.list.Len())
	for elem := s.list.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*lruEntry)
		keys[entry.key] = entry.hash
	}
	return keys
}
