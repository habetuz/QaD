package storage

type Storage interface {
	Read(key string) []byte
	Write(key string, hash uint64, value []byte)
	Delete(key string)
	ListKeys() map[string]uint64
}
