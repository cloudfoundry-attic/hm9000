package shredder_test

import (
	"github.com/cloudfoundry/hm9000/config"
	. "github.com/cloudfoundry/hm9000/shredder"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Shredder", func() {
	var (
		shredder     *Shredder
		storeAdapter *fakestoreadapter.FakeStoreAdapter
	)

	BeforeEach(func() {
		storeAdapter = fakestoreadapter.New()
		conf, _ := config.DefaultConfig()
		conf.StoreSchemaVersion = 2
		store := storepackage.NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
		shredder = New(store, fakelogger.NewFakeLogger())

		storeAdapter.SetMulti([]storeadapter.StoreNode{
			{Key: "/hm/v2/pokemon/geodude", Value: []byte{}},
			{Key: "/hm/v2/deep-pokemon/abra/kadabra/alakazam", Value: []byte{}},
			{Key: "/hm/v2/pokemonCount", Value: []byte("151")},
			{Key: "/hm/v1/nuke/me/cause/im/an/old/version", Value: []byte("abc")},
			{Key: "/hm/v3/leave/me/alone/since/im/a/new/version", Value: []byte("abc")},
			{Key: "/hm/nuke/me/cause/im/not/versioned", Value: []byte("abc")},
			{Key: "/let/me/be", Value: []byte("abc")},
		})

		storeAdapter.Delete("/hm/v2/pokemon/geodude", "/hm/v2/deep-pokemon/abra/kadabra/alakazam")
		err := shredder.Shred()
		Expect(err).ToNot(HaveOccurred())
	})

	It("should delete empty directories", func() {
		_, err := storeAdapter.Get("/hm/v2/pokemon")
		Expect(err.(storeadapter.Error).Type()).To(Equal(storeadapter.ErrorKeyNotFound))

		_, err = storeAdapter.Get("/hm/v2/deep-pokemon")
		Expect(err.(storeadapter.Error).Type()).To(Equal(storeadapter.ErrorKeyNotFound))

		_, err = storeAdapter.Get("/hm/v2/pokemonCount")
		Expect(err).ToNot(HaveOccurred())
	})

	It("should delete everything underneath older versions", func() {
		_, err := storeAdapter.Get("/hm/v1/nuke/me/cause/im/an/old/version")
		Expect(err.(storeadapter.Error).Type()).To(Equal(storeadapter.ErrorKeyNotFound))
	})

	It("should delete everything that is not versioned", func() {
		_, err := storeAdapter.Get("/hm/nuke/me/cause/im/not/versioned")
		Expect(err.(storeadapter.Error).Type()).To(Equal(storeadapter.ErrorKeyNotFound))
	})

	It("should not delete newer versions", func() {
		_, err := storeAdapter.Get("/hm/v3/leave/me/alone/since/im/a/new/version")
		Expect(err).ToNot(HaveOccurred())
	})

	It("should not delete anything that isn't under the hm namespace", func() {
		_, err := storeAdapter.Get("/let/me/be")
		Expect(err).ToNot(HaveOccurred())
	})
})
