package storerunner

import (
	"fmt"
	. "github.com/onsi/gomega"

	"io/ioutil"
	"os"
	"os/exec"
)

//See http://zookeeper.apache.org/doc/trunk/zookeeperStarted.html

type ZookeeperClusterRunner struct {
	startingPort int
	numNodes     int
	running      bool
}

func NewZookeeperClusterRunner(startingPort int, numNodes int) *ZookeeperClusterRunner {
	return &ZookeeperClusterRunner{
		startingPort: startingPort,
		numNodes:     numNodes,
	}
}

func (zk *ZookeeperClusterRunner) Start() {
	for i := 0; i < zk.numNodes; i++ {
		zk.nukeArtifacts(i)
		os.MkdirAll(zk.tmpPath(i), 0700)
		zk.writeId(i)
		zk.writeConfig(i)

		cmd := exec.Command("zkServer.sh", "start", zk.configPath(i))
		cmd.Env = append(os.Environ(), "ZOO_LOG_DIR="+zk.tmpPath(i))

		out, err := cmd.Output()
		Ω(err).ShouldNot(HaveOccured(), "Make sure zookeeper is compiled and on your $PATH.")
		Ω(out).Should(ContainSubstring("STARTED"))

		Eventually(func() interface{} {
			return zk.exists(i)
		}, 3, 0.05).Should(BeTrue(), "Expected Zookeeper to be up and running")
	}
	zk.running = true
}

func (zk *ZookeeperClusterRunner) Stop() {
	if zk.running {
		for i := 0; i < zk.numNodes; i++ {
			cmd := exec.Command("zkServer.sh", "stop", zk.configPath(i))
			out, err := cmd.Output()

			Ω(err).ShouldNot(HaveOccured(), "Zookeeper failed to stop!")
			Ω(out).Should(ContainSubstring("STOPPED"))

			zk.nukeArtifacts(i)
		}
		zk.running = false
	}
}

func (zk *ZookeeperClusterRunner) NodeURLS() []string {
	urls := make([]string, zk.numNodes)
	for i := 0; i < zk.numNodes; i++ {
		urls[i] = "http://" + zk.clientUrl(i)
	}
	return urls
}

func (zk *ZookeeperClusterRunner) DiskUsage() (bytes int64, err error) {
	fi, err := os.Stat(zk.tmpPathTo("version-2/snapshot.0", 0))
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func (zk *ZookeeperClusterRunner) Reset() {
	// client := zkclient.NewClient()
	// client.SetCluster(zk.NodeURLS())

	// zk.deleteDir(client, "/")
}

// func (zk *ZookeeperClusterRunner) deleteDir(client *zkclient.Client, dir string) {
// responses, err := client.Get(dir)
// Ω(err).ShouldNot(HaveOccured())
// for _, response := range responses {
// 	if response.Key != "/_zk" {
// 		if response.Dir == true {
// 			zk.deleteDir(client, response.Key)
// 		} else {
// 			_, err := client.Delete(response.Key)
// 			Ω(err).ShouldNot(HaveOccured())
// 		}
// 	}
// }
// }

func (zk *ZookeeperClusterRunner) writeConfig(index int) {
	config := "tickTime=2000\n"
	config += fmt.Sprintf("dataDir=%s\n", zk.tmpPath(index))
	config += fmt.Sprintf("clientPort=%d\n", zk.clientPort(index))

	if zk.numNodes > 1 {
		config += "initLimit=5\n"
		config += "syncLimit=2\n"
		for node := 1; node <= zk.numNodes; node++ {
			config += fmt.Sprintf("server.%d=127.0.0.1:%d:%d\n", node, zk.serverPort(node), zk.electionPort(node))
		}
	}

	err := ioutil.WriteFile(zk.configPath(index), []byte(config), 0700)
	Ω(err).ShouldNot(HaveOccured())
}

func (zk *ZookeeperClusterRunner) writeId(index int) {
	err := ioutil.WriteFile(zk.tmpPathTo("myid", index), []byte(fmt.Sprintf("%d", index+1)), 0700)
	Ω(err).ShouldNot(HaveOccured())
}

func (zk *ZookeeperClusterRunner) clientUrl(index int) string {
	return fmt.Sprintf("127.0.0.1:%d", zk.clientPort(index))
}

func (zk *ZookeeperClusterRunner) clientPort(index int) int {
	return zk.startingPort + index
}

func (zk *ZookeeperClusterRunner) serverPort(index int) int {
	return zk.startingPort + index + 707
}

func (zk *ZookeeperClusterRunner) electionPort(index int) int {
	return zk.startingPort + index + 1707
}

func (zk *ZookeeperClusterRunner) tmpPath(index int) string {
	return fmt.Sprintf("/tmp/ZOOKEEPER_%d", zk.clientPort(index))
}

func (zk *ZookeeperClusterRunner) configPath(index int) string {
	return zk.tmpPath(index) + ".conf"
}

func (zk *ZookeeperClusterRunner) tmpPathTo(subdir string, index int) string {
	return fmt.Sprintf("/%s/%s", zk.tmpPath(index), subdir)
}

func (zk *ZookeeperClusterRunner) nukeArtifacts(index int) {
	os.RemoveAll(zk.tmpPath(index))
	os.Remove(zk.configPath(index))
}

func (zk *ZookeeperClusterRunner) exists(index int) bool {
	_, err := os.Stat(zk.tmpPathTo("zookeeper_server.pid", index))
	return err == nil
}
