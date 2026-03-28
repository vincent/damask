package storage

import "io"

// Storage is the interface that wraps basic asset storage operations.
// LocalStorage is the default implementation. Swap for S3Storage at config level.
type Storage interface {
	Put(key string, r io.Reader) error
	Get(key string) (io.ReadCloser, error)
	Delete(key string) error
	List(prefix string) ([]string, error)
}
