package cfcomponent_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCfcomponent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cfcomponent Suite")
}
