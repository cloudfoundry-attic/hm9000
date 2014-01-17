package storeadapter

import (
	"github.com/cloudfoundry/hm9000/helpers/workerpool"
	"github.com/coreos/go-etcd/etcd"
	"github.com/nu7hatch/gouuid"
	"path"
	"time"
)

type ETCDStoreAdapter struct {
	urls       []string
	client     *etcd.Client
	workerPool *workerpool.WorkerPool
}

func NewETCDStoreAdapter(urls []string, workerPool *workerpool.WorkerPool) *ETCDStoreAdapter {
	return &ETCDStoreAdapter{
		urls:       urls,
		workerPool: workerPool,
	}
}

func (adapter *ETCDStoreAdapter) Connect() error {
	adapter.client = etcd.NewClient(adapter.urls)

	return nil
}

func (adapter *ETCDStoreAdapter) Disconnect() error {
	adapter.workerPool.StopWorkers()

	return nil
}

func (adapter *ETCDStoreAdapter) isETCDErrorWithCode(err error, code int) bool {
	if err != nil {
		etcdError, ok := err.(etcd.EtcdError)
		if ok && etcdError.ErrorCode == code {
			return true
		}

		etcdErrorP, ok := err.(*etcd.EtcdError)
		if ok && etcdErrorP.ErrorCode == code {
			return true
		}
	}
	return false
}

func (adapter *ETCDStoreAdapter) isTimeoutError(err error) bool {
	return adapter.isETCDErrorWithCode(err, 501)
}

func (adapter *ETCDStoreAdapter) isMissingKeyError(err error) bool {
	return adapter.isETCDErrorWithCode(err, 100)
}

func (adapter *ETCDStoreAdapter) isNotAFileError(err error) bool {
	return adapter.isETCDErrorWithCode(err, 102)
}

func (adapter *ETCDStoreAdapter) Set(nodes []StoreNode) error {
	results := make(chan error, len(nodes))

	for _, node := range nodes {
		node := node
		adapter.workerPool.ScheduleWork(func() {
			_, err := adapter.client.Set(node.Key, string(node.Value), node.TTL)
			results <- err
		})
	}

	var err error
	numReceived := 0
	for numReceived < len(nodes) {
		result := <-results
		numReceived++
		if err == nil {
			err = result
		}
	}

	if adapter.isNotAFileError(err) {
		return ErrorNodeIsDirectory
	}

	if adapter.isTimeoutError(err) {
		return ErrorTimeout
	}

	return err
}

func (adapter *ETCDStoreAdapter) Get(key string) (StoreNode, error) {
	done := make(chan bool, 1)
	var response *etcd.Response
	var err error

	//we route through the worker pool to enable usage tracking
	adapter.workerPool.ScheduleWork(func() {
		response, err = adapter.client.Get(key, false, false)
		done <- true
	})

	<-done

	if adapter.isTimeoutError(err) {
		return StoreNode{}, ErrorTimeout
	}

	if adapter.isMissingKeyError(err) {
		return StoreNode{}, ErrorKeyNotFound
	}
	if err != nil {
		return StoreNode{}, err
	}

	if response.Node.Dir {
		return StoreNode{}, ErrorNodeIsDirectory
	}

	return StoreNode{
		Key:   response.Node.Key,
		Value: []byte(response.Node.Value),
		Dir:   response.Node.Dir,
		TTL:   uint64(response.Node.TTL),
	}, nil
}

func (adapter *ETCDStoreAdapter) ListRecursively(key string) (StoreNode, error) {
	done := make(chan bool, 1)
	var response *etcd.Response
	var err error

	//we route through the worker pool to enable usage tracking
	adapter.workerPool.ScheduleWork(func() {
		response, err = adapter.client.Get(key, false, true)
		done <- true
	})

	<-done

	if adapter.isTimeoutError(err) {
		return StoreNode{}, ErrorTimeout
	}

	if adapter.isMissingKeyError(err) {
		return StoreNode{}, ErrorKeyNotFound
	}

	if err != nil {
		return StoreNode{}, err
	}

	if !response.Node.Dir {
		return StoreNode{}, ErrorNodeIsNotDirectory
	}

	if len(response.Node.Nodes) == 0 {
		return StoreNode{Key: key, Dir: true, Value: []byte{}, ChildNodes: []StoreNode{}}, nil
	}

	return adapter.makeStoreNode(*response.Node), nil
}

func (adapter *ETCDStoreAdapter) makeStoreNode(etcdNode etcd.Node) StoreNode {
	if etcdNode.Dir {
		node := StoreNode{
			Key:        etcdNode.Key,
			Dir:        true,
			Value:      []byte{},
			ChildNodes: []StoreNode{},
		}

		for _, child := range etcdNode.Nodes {
			node.ChildNodes = append(node.ChildNodes, adapter.makeStoreNode(child))
		}

		return node
	} else {
		return StoreNode{
			Key:   etcdNode.Key,
			Value: []byte(etcdNode.Value),
			TTL:   uint64(etcdNode.TTL),
		}
	}
}

func (adapter *ETCDStoreAdapter) Delete(keys ...string) error {
	results := make(chan error, len(keys))

	for _, key := range keys {
		key := key
		adapter.workerPool.ScheduleWork(func() {
			_, err := adapter.client.Delete(key, true)
			results <- err
		})
	}

	var err error
	numReceived := 0
	for numReceived < len(keys) {
		result := <-results
		numReceived++
		if err == nil {
			err = result
		}
	}

	if adapter.isTimeoutError(err) {
		return ErrorTimeout
	}

	if adapter.isMissingKeyError(err) {
		return ErrorKeyNotFound
	}

	return err
}

func (adapter *ETCDStoreAdapter) lockKey(lockName string) string {
	return path.Join("/hm/locks", lockName)
}

func (adapter *ETCDStoreAdapter) GetAndMaintainLock(lockName string, lockTTL uint64, lostLockChannel chan bool) (releaseLock chan bool, err error) {
	if lockTTL == 0 {
		return nil, ErrorInvalidTTL
	}

	guid, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	currentLockValue := guid.String()

	lockKey := adapter.lockKey(lockName)

	releaseLockChannel := make(chan bool, 0)

	for {
		_, err := adapter.client.Create(lockKey, currentLockValue, lockTTL)
		if adapter.isTimeoutError(err) {
			return nil, ErrorTimeout
		}

		if err == nil {
			break
		}

		time.Sleep(1 * time.Second)
	}

	go adapter.maintainLock(lockKey, currentLockValue, lockTTL, lostLockChannel, releaseLockChannel)

	return releaseLockChannel, nil
}

func (adapter *ETCDStoreAdapter) maintainLock(lockKey string, currentLockValue string, lockTTL uint64, lostLockChannel chan bool, releaseLockChannel chan bool) {
	maintenanceInterval := time.Duration(lockTTL) * time.Second / time.Duration(2)
	ticker := time.NewTicker(maintenanceInterval)
	for {
		select {
		case <-ticker.C:
			_, err := adapter.client.CompareAndSwap(lockKey, currentLockValue, lockTTL, currentLockValue, 0)
			if err != nil {
				lostLockChannel <- true
			}
		case <-releaseLockChannel:
			adapter.client.CompareAndSwap(lockKey, currentLockValue, 1, currentLockValue, 0)
			return
		}
	}
}
