package storage

var _ Storage = (*NoEvictionStorage)(nil)

type Storage interface {
	Read(key string) []byte
	Write(key string, value []byte)
	Delete(key string)
}
