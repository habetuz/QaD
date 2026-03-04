package storage

import (
	"container/list"
	"sync"

	"github.com/rs/zerolog/log"
)

var _ Storage = (*LRUStorage)(nil)

type lruEntry struct {
	key   string
	value []byte
}

// LRUStorage evicts the least-recently-used key when the size limit is exceeded.
// Read promotes the accessed entry (mutating list order), so a full Mutex is used
// rather than RWMutex.
type LRUStorage struct {
	mu      sync.Mutex
	storage map[string]*list.Element
	list    *list.List // front = most recent, back = least recent
	maxSize int
	curSize int
}

func NewLRUStorage(maxSize int) *LRUStorage {
	return &LRUStorage{
		storage: make(map[string]*list.Element),
		list:    list.New(),
		maxSize: maxSize,
	}
}

// Read implements [Storage].
func (s *LRUStorage) Read(key string) []byte {
	el, ok := s.storage[key]
	if !ok {
		log.Debug().Str("key", key).Bool("exists", false).Msg("LRU read key")
		return nil
	}
	go func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.list.MoveToFront(el)
	}()
	value := el.Value.(*lruEntry).value
	log.Debug().Str("key", key).Bool("exists", true).Int("length", len(value)).Msg("LRU read key")
	return value
}

// Write implements [Storage].
func (s *LRUStorage) Write(key string, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if el, ok := s.storage[key]; ok {
		s.curSize -= len(el.Value.(*lruEntry).value)
		el.Value.(*lruEntry).value = value
		s.list.MoveToFront(el)
	} else {
		el = s.list.PushFront(&lruEntry{key: key, value: value})
		s.storage[key] = el
	}
	s.curSize += len(value)

	// Evict least-recently-used entries until within size limit.
	for s.curSize > s.maxSize && s.list.Len() > 0 {
		lru := s.list.Back()
		entry := lru.Value.(*lruEntry)
		s.curSize -= len(entry.value)
		delete(s.storage, entry.key)
		s.list.Remove(lru)
		log.Debug().Str("key", entry.key).Msg("LRU evicted key")
	}

	log.Debug().Str("key", key).Int("length", len(value)).Msg("LRU wrote key")
}

// Delete implements [Storage].
func (s *LRUStorage) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if el, ok := s.storage[key]; ok {
		s.curSize -= len(el.Value.(*lruEntry).value)
		s.list.Remove(el)
		delete(s.storage, key)
	}
	log.Debug().Str("key", key).Msg("LRU deleted key")
}
