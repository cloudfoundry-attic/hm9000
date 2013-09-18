package storerunner

type StoreRunner interface {
	Start()
	Stop()
	NodeURLS() []string
	DiskUsage() int64
	Reset()
}
