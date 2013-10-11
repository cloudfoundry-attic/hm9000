package metricsserver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMetricsserver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metricsserver Suite")
}
