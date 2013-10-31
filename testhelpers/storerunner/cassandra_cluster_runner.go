package storerunner

import (
	"fmt"
	. "github.com/onsi/gomega"
	"os/exec"
	"tux21b.org/v1/gocql"
)

type CassandraClusterRunner struct {
	port             int
	cassandraCommand *exec.Cmd
}

func NewCassandraClusterRunner(port int) *CassandraClusterRunner {
	return &CassandraClusterRunner{
		port: port,
	}
}

func (c *CassandraClusterRunner) Start() {
	c.cassandraCommand = exec.Command("cassandra", "-f")
	err := c.cassandraCommand.Start()
	Ω(err).ShouldNot(HaveOccured())

	cluster := gocql.NewCluster("127.0.0.1")
	cluster.DefaultPort = c.port
	cluster.Consistency = gocql.One
	session, err := cluster.CreateSession()
	defer session.Close()

	Eventually(func() error {
		return session.Query(`select * from system.schema_keyspaces`).Exec()
	}).ShouldNot(HaveOccured())
}

func (c *CassandraClusterRunner) Stop() {
	c.cassandraCommand.Process.Kill()
}

func (c *CassandraClusterRunner) NodeURLS() []string {
	return []string{fmt.Sprintf("127.0.0.1:%d", c.port)}
}

func (c *CassandraClusterRunner) DiskUsage() (bytes int64, err error) {
	return 0, nil
}

func (c *CassandraClusterRunner) FastForwardTime(seconds int) {
}

func (c *CassandraClusterRunner) Reset() {
	cluster := gocql.NewCluster(c.NodeURLS()...)
	cluster.DefaultPort = c.port
	cluster.Consistency = gocql.One
	session, err := cluster.CreateSession()
	Ω(err).ShouldNot(HaveOccured())
	defer session.Close()
	err = session.Query(`DROP KEYSPACE IF EXISTS hm9000`).Exec()
	Ω(err).ShouldNot(HaveOccured())
}
