package storeadapter

type ETCDErrorReason string

const (
	ETCDErrorInvalid        ETCDErrorReason = ""
	ETCDErrorKeyNotFound    ETCDErrorReason = "KeyNotFound"
	ETCDErrorIsDirectory    ETCDErrorReason = "IsDirectory"
	ETCDErrorIsNotDirectory ETCDErrorReason = "IsNotDirectory"
	ETCDErrorTimeout        ETCDErrorReason = "Timeout Reaching Store"
)

type ETCDError struct {
	reason ETCDErrorReason
}

func (err ETCDError) Error() string {
	return string(err.reason)
}

func IsKeyNotFoundError(err error) bool {
	etcdErr, ok := err.(ETCDError)
	if !ok {
		return false
	}
	return etcdErr.reason == ETCDErrorKeyNotFound
}

func IsDirectoryError(err error) bool {
	etcdErr, ok := err.(ETCDError)
	if !ok {
		return false
	}
	return etcdErr.reason == ETCDErrorIsDirectory
}

func IsNotDirectoryError(err error) bool {
	etcdErr, ok := err.(ETCDError)
	if !ok {
		return false
	}
	return etcdErr.reason == ETCDErrorIsNotDirectory
}

func IsTimeoutError(err error) bool {
	etcdErr, ok := err.(ETCDError)
	if !ok {
		return false
	}
	return etcdErr.reason == ETCDErrorTimeout
}
