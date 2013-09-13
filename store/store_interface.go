package store

type Store interface {
	Connect() error
	Set(key string, value []byte, ttl uint64) error
	Get(key string) (StoreNode, error)
	List(key string) ([]StoreNode, error)
	Delete(key string) error
}

type StoreNode struct {
	Key   string
	Value []byte
	Dir   bool
	TTL   uint64
}
