package store

type Store interface {
	Connect() error
	Set(nodes []StoreNode) error
	Get(key string) (StoreNode, error)
	List(key string) ([]StoreNode, error)
	Delete(key string) error
	Disconnect() error
}

type StoreNode struct {
	Key   string
	Value []byte
	Dir   bool
	TTL   uint64
}
