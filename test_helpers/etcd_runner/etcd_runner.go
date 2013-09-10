package etcd_runner

import (
	"fmt"
	. "github.com/onsi/gomega"

	"os"
	"os/exec"
	"syscall"
)

type ETCDRunner struct {
	path        string
	port        int
	etcdCommand *exec.Cmd
}

func NewETCDRunner(path string, port int) *ETCDRunner {
	return &ETCDRunner{
		path: path,
		port: port,
	}
}

func (etcd *ETCDRunner) StartETCD() {
	os.MkdirAll(etcd.tmpPath(), 0700)
	etcd.etcdCommand = exec.Command(etcd.path, "-d", etcd.tmpPath(), "-c", fmt.Sprintf("127.0.0.1:%d", etcd.port))

	err := etcd.etcdCommand.Start()
	Ω(err).ShouldNot(HaveOccured(), "Make sure etcd is compiled and on your $PATH.")

	Eventually(func() interface{} {
		return etcd.exists()
	}, 1, 0.05).Should(BeTrue(), "Expected ETCD")
}

func (etcd *ETCDRunner) StopETCD() {
	if etcd.etcdCommand != nil {
		etcd.etcdCommand.Process.Signal(syscall.SIGINT)
		etcd.etcdCommand.Process.Wait()
		etcd.etcdCommand = nil
		os.Remove(etcd.tmpPathTo("log"))
		os.Remove(etcd.tmpPathTo("info"))
		os.Remove(etcd.tmpPathTo("snapshot"))
		os.Remove(etcd.tmpPathTo("conf"))
	}
}

func (etcd *ETCDRunner) tmpPath() string {
	return fmt.Sprintf("/tmp/ETCD_%d", etcd.port)
}

func (etcd *ETCDRunner) tmpPathTo(subdir string) string {
	return fmt.Sprintf("/%s/%s", etcd.tmpPath(), subdir)
}

func (etcd *ETCDRunner) exists() bool {
	_, err := os.Stat(etcd.tmpPathTo("info"))
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}
