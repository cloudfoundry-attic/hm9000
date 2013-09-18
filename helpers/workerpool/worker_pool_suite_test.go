package workerpool

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestWorkerPool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker Pool Suite")
}
