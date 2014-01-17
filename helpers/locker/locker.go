package locker

import (
	"fmt"
	"path"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/nu7hatch/gouuid"
)

var NoTTLError = fmt.Errorf("lock must have a nonzero TTL")
var NoStoreError = fmt.Errorf("could not reach etcd")

type Locker interface {
	GetAndMaintainLock(lostLockChannel chan bool) error
	ReleaseLock()
}

type ETCDLocker struct {
	etcdClient *etcd.Client
	lockName   string
	lockTTL    uint64

	currentLockValue string

	stopMaintaining chan bool
}

func New(
	etcdClient *etcd.Client, lockName string, lockTTL uint64,
) *ETCDLocker {
	guid, err := uuid.NewV4()
	if err != nil {
		panic("failed to construct uuid: " + err.Error())
	}

	return &ETCDLocker{
		etcdClient: etcdClient,
		lockName:   lockName,
		lockTTL:    lockTTL,

		currentLockValue: guid.String(),
		stopMaintaining:  make(chan bool),
	}
}

func (l *ETCDLocker) GetAndMaintainLock(lostLockChannel chan bool) error {
	if l.lockTTL == 0 {
		return NoTTLError
	}

	res, err := l.etcdClient.Get(l.lockKey(), false, false)
	if err == nil && res.Node.Value == l.currentLockValue {
		return nil
	}

	for {
		_, err := l.etcdClient.Create(l.lockKey(), l.currentLockValue, l.lockTTL)
		if l.isTimeoutError(err) {
			return NoStoreError
		}

		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		go l.maintainLock(lostLockChannel)

		break
	}

	return nil
}

func (l *ETCDLocker) ReleaseLock() {
	l.stopMaintaining <- true
}

func (l *ETCDLocker) maintainLock(lostLockChannel chan bool) {
	maintenanceInterval := time.Duration(l.lockTTL) * time.Second / time.Duration(2)
	ticker := time.NewTicker(maintenanceInterval)

Dance:
	for {
		select {
		case <-ticker.C:
			_, err := l.etcdClient.CompareAndSwap(l.lockKey(), l.currentLockValue, l.lockTTL, l.currentLockValue, 0)
			if err != nil {
				lostLockChannel <- true
			}
		case <-l.stopMaintaining:
			l.etcdClient.CompareAndSwap(l.lockKey(), l.currentLockValue, 1, l.currentLockValue, 0)
			break Dance
		}
	}
}

func (l *ETCDLocker) lockKey() string {
	return path.Join("/hm/locks", l.lockName)
}

func (l *ETCDLocker) isTimeoutError(err error) bool {
	if err != nil {
		etcdError, ok := err.(etcd.EtcdError)
		if ok && etcdError.ErrorCode == 501 {
			return true
		}

		etcdErrorP, ok := err.(*etcd.EtcdError)
		if ok && etcdErrorP.ErrorCode == 501 {
			return true
		}
	}
	return false
}
