package storeadapter

type StoreAdapter interface {
	Connect() error
	Set(nodes []StoreNode) error
	Get(key string) (StoreNode, error)
	ListRecursively(key string) (StoreNode, error)
	Delete(keys ...string) error
	Disconnect() error

	GetAndMaintainLock(lockName string, lockTTL uint64) (lostLock <-chan bool, releaseLock chan<- bool, err error)
}

type StoreNode struct {
	Key        string
	Value      []byte
	Dir        bool
	TTL        uint64
	ChildNodes []StoreNode
}
