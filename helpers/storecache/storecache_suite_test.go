package storecache_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestStorecache(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storecache Suite")
}
