package storage

import "github.com/rs/zerolog/log"

var _ Storage = (*InMemoryStorage)(nil)

type Storage interface {
	Read(key string) []byte
	Write(key string, value []byte)
	Delete(key string)
}

type InMemoryStorage struct {
	storage map[string][]byte
	maxSize uint
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{storage: make(map[string][]byte)}
}

// Delete implements [Storage].
func (s *InMemoryStorage) Delete(key string) {
	delete(s.storage, key)
	log.Debug().Str("key", key).Msg("Deleted key")
}

// Read implements [Storage].
func (s *InMemoryStorage) Read(key string) []byte {
	value, ok := s.storage[key]
	log.Debug().Str("key", key).Bool("exists", ok).Int("length", len(value)).Msg("Read key")
	if ok {
		return value
	} else {
		return nil
	}
}

// Write implements [Storage].
func (s *InMemoryStorage) Write(key string, value []byte) {
	s.storage[key] = value
	log.Debug().Str("key", key).Int("length", len(value)).Msg("Wrote key")
}
