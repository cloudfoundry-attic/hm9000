package fakelocker

type FakeLocker struct {
	GetAndMaintainLockError error
	GotAndMaintainedLock    bool
	LostLockChannel         chan bool
	ReleasedLock            bool
}

func New() *FakeLocker {
	return &FakeLocker{}
}

func (l *FakeLocker) GetAndMaintainLock(lostLockChannel chan bool) error {
	if l.GetAndMaintainLockError != nil {
		return l.GetAndMaintainLockError
	}

	l.GotAndMaintainedLock = true
	l.LostLockChannel = lostLockChannel

	return nil
}

func (l *FakeLocker) ReleaseLock() {
	l.ReleasedLock = true
}
