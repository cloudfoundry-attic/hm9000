package store

type Store interface {
	Connect() error
	Set(key string, value string, ttl uint64) error
	Get(key string) (string, error)
	List(key string) ([]StoreNode, error)
	Delete(key string) error

	// IsMissing(err error) bool
	// IsDirectory(err error) bool
}

type StoreNode struct {
	Key   string
	Value string
	Dir   bool
}
