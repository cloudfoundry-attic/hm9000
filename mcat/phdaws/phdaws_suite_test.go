package phd_aws

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/storecassandra"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var store *storecassandra.StoreCassandra

func TestPhdAWS(t *testing.T) {
	RegisterFailHandler(Fail)

	conf, _ := config.DefaultConfig()
	var err error
	store, err = storecassandra.New([]string{"127.0.0.1:9042"}, conf, timeprovider.NewTimeProvider())
	Ω(err).ShouldNot(HaveOccured())

	RunSpecsWithDefaultAndCustomReporters(t, "MCAT AWS PhD Suite", []Reporter{&DataReporter{Title: "Local_ETCD"}})
}
