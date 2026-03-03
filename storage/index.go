package storage

var _ Storage = (*InMemoryStorage)(nil)

type Storage interface {
	Read(key string) *[]byte
	Write(key string, value *[]byte)
	Delete(key string)
}

type InMemoryStorage struct {
	storage map[string]byte
	maxSize uint
}

// Delete implements [Storage].
func (s *InMemoryStorage) Delete(key string) {
	delete(s.storage, key)
}

// Read implements [Storage].
func (s *InMemoryStorage) Read(key string) *[]byte {
	panic("unimplemented")
}

// Write implements [Storage].
func (s *InMemoryStorage) Write(string, *[]byte) {
	panic("unimplemented")
}
