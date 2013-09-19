package storeadapter

type StoreErrorReason string

const (
	StoreErrorInvalid        StoreErrorReason = ""
	StoreErrorKeyNotFound    StoreErrorReason = "KeyNotFound"
	StoreErrorIsDirectory    StoreErrorReason = "IsDirectory"
	StoreErrorIsNotDirectory StoreErrorReason = "IsNotDirectory"
	StoreErrorTimeout        StoreErrorReason = "Timeout Reaching Store"
)

type StoreError struct {
	reason StoreErrorReason
}

func NewStoreError(reason StoreErrorReason) StoreError {
	return StoreError{reason: reason}
}

func (err StoreError) Error() string {
	return string(err.reason)
}

func IsKeyNotFoundError(err error) bool {
	etcdErr, ok := err.(StoreError)
	if !ok {
		return false
	}
	return etcdErr.reason == StoreErrorKeyNotFound
}

func IsDirectoryError(err error) bool {
	etcdErr, ok := err.(StoreError)
	if !ok {
		return false
	}
	return etcdErr.reason == StoreErrorIsDirectory
}

func IsNotDirectoryError(err error) bool {
	etcdErr, ok := err.(StoreError)
	if !ok {
		return false
	}
	return etcdErr.reason == StoreErrorIsNotDirectory
}

func IsTimeoutError(err error) bool {
	etcdErr, ok := err.(StoreError)
	if !ok {
		return false
	}
	return etcdErr.reason == StoreErrorTimeout
}
