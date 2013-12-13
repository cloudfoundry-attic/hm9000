package locker

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/nu7hatch/gouuid"
)

var NoTTLError = fmt.Errorf("lock must have a nonzero TTL")
var NoStoreError = fmt.Errorf("could not reach etcd")

type Locker struct {
	etcdClient *etcd.Client
	lockName   string
	lockTTL    uint64

	currentLockValue string

	stopMaintaining chan bool
}

func New(
	etcdClient *etcd.Client, lockName string, lockTTL uint64,
) *Locker {
	guid, err := uuid.NewV4()
	if err != nil {
		panic("failed to construct uuid: " + err.Error())
	}

	return &Locker{
		etcdClient: etcdClient,
		lockName:   lockName,
		lockTTL:    lockTTL,

		currentLockValue: guid.String(),
		stopMaintaining:  make(chan bool),
	}
}

func (l *Locker) GetAndMaintainLock() error {
	if l.lockTTL == 0 {
		return NoTTLError
	}

	res, err := l.etcdClient.Get(l.lockKey(), false, false)
	if err == nil && res.Node.Value == l.currentLockValue {
		return nil
	}

	for {
		_, err := l.etcdClient.Create(l.lockKey(), l.currentLockValue, l.lockTTL)
		if err != nil && strings.HasPrefix(err.Error(), "Cannot reach servers") {
			return NoStoreError
		}

		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		go l.maintainLock()

		break
	}

	return nil
}

func (l *Locker) ReleaseLock() {
	l.stopMaintaining <- true
}

func (l *Locker) maintainLock() {
	maintenanceInterval := time.Duration(l.lockTTL) * time.Second / time.Duration(2)
	ticker := time.NewTicker(maintenanceInterval)

Dance:
	for {
		select {
		case <-ticker.C:
			l.etcdClient.CompareAndSwap(l.lockKey(), l.currentLockValue, l.lockTTL, l.currentLockValue, 0)
		case <-l.stopMaintaining:
			l.etcdClient.CompareAndSwap(l.lockKey(), l.currentLockValue, 1, l.currentLockValue, 0)
			break Dance
		}
	}
}

func (l *Locker) lockKey() string {
	return path.Join("locks", l.lockName)
}
