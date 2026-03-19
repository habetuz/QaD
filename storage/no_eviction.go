package storage

import "github.com/rs/zerolog/log"

var _ Storage = (*NoEvictionStorage)(nil)

type NoEvictionStorage struct {
	storage map[string][]byte
	hashes  map[string]uint64
	maxSize uint
}

func NewNoEvictionStorage() *NoEvictionStorage {
	return &NoEvictionStorage{
		storage: make(map[string][]byte),
		hashes:  make(map[string]uint64),
	}
}

// Delete implements [Storage].
func (s *NoEvictionStorage) Delete(key string) {
	delete(s.storage, key)
	delete(s.hashes, key)
	log.Debug().Str("key", key).Msg("Deleted key")
}

// Read implements [Storage].
func (s *NoEvictionStorage) Read(key string) []byte {
	value, ok := s.storage[key]
	log.Debug().Str("key", key).Bool("exists", ok).Int("length", len(value)).Msg("Read key")
	if ok {
		return value
	} else {
		return nil
	}
}

// Write implements [Storage].
func (s *NoEvictionStorage) Write(key string, hash uint64, value []byte) {
	s.storage[key] = value
	s.hashes[key] = hash
	log.Debug().Str("key", key).Int("length", len(value)).Msg("Wrote key")
}

// ListKeys implements [Storage].
func (s *NoEvictionStorage) ListKeys() map[string]uint64 {
	return s.hashes
}
