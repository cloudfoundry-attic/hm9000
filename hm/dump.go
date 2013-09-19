package hm

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/storeadapter"

	"github.com/codegangsta/cli"
)

func Dump(l logger.Logger, c *cli.Context) {
	conf := loadConfig(l, c)
	etcdStoreAdapter := connectToETCDStoreAdapter(l, conf)

	walk(etcdStoreAdapter, "/", func(node storeadapter.StoreNode) {
		ttl := fmt.Sprintf("[TTL:%ds]", node.TTL)
		if node.TTL == 0 {
			ttl = "[TTL: ∞]"
		}
		fmt.Printf("%s %s: %s\n", node.Key, ttl, node.Value)
	})
}

func walk(store storeadapter.StoreAdapter, dirKey string, callback func(storeadapter.StoreNode)) {
	nodes, err := store.List(dirKey)
	if err != nil {
		return
	}

	for _, node := range nodes {
		if node.Key == "/_etcd" {
			continue
		}
		if node.Dir {
			walk(store, node.Key, callback)
		} else {
			callback(node)
		}
	}
}
