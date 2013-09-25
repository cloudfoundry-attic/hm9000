package hm

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"sort"
	"time"

	"github.com/codegangsta/cli"
)

func Dump(l logger.Logger, c *cli.Context) {
	conf := loadConfig(l, c)
	etcdStoreAdapter := connectToETCDStoreAdapter(l, conf)
	fmt.Printf("Dump - Current timestamp %d\n", time.Now().Unix())

	entries := sort.StringSlice{}
	Walk(etcdStoreAdapter, "/", func(node storeadapter.StoreNode) {
		ttl := fmt.Sprintf("[TTL:%ds]", node.TTL)
		if node.TTL == 0 {
			ttl = "[TTL: ∞]"
		}
		entries = append(entries, fmt.Sprintf("%s %s:\n    %s", node.Key, ttl, node.Value))
	})

	sort.Sort(entries)
	previousEntry := "/aaa"
	for _, entry := range entries {
		if previousEntry[0:3] < entry[0:3] {
			fmt.Printf("\n")
		}
		fmt.Printf(entry + "\n")
		previousEntry = entry
	}
}

func Walk(store storeadapter.StoreAdapter, dirKey string, callback func(storeadapter.StoreNode)) {
	nodes, err := store.List(dirKey)
	if err != nil {
		return
	}

	for _, node := range nodes {
		if node.Key == "/_etcd" {
			continue
		}
		if node.Dir {
			Walk(store, node.Key, callback)
		} else {
			callback(node)
		}
	}
}
