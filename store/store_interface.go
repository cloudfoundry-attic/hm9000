package store

type Store interface {
	Connect() error
	Set(key string, value string, ttl uint64) error
	Get(key string) (StoreNode, error)
	List(key string) ([]StoreNode, error)
	Delete(key string) error
}

type StoreNode struct {
	Key   string
	Value string
	Dir   bool
	TTL   uint64
}
