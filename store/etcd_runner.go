package store

import (
	. "github.com/onsi/gomega"

	"os"
	"os/exec"
	"syscall"
)

type etcdRunner struct {
	path        string
	etcdCommand *exec.Cmd
}

func NewETCDRunner(path string) *etcdRunner {
	return &etcdRunner{
		path: path,
	}
}

func (etcd *etcdRunner) StartETCD() {
	etcd.etcdCommand = exec.Command(etcd.path, "-d", "/tmp")

	err := etcd.etcdCommand.Start()
	Ω(err).ShouldNot(HaveOccured(), "Make sure etcd is compiled and on your $PATH.")
	Eventually(func() interface{} {
		return etcd.exists()
	}, 1, 0.05).Should(BeTrue())
}

func (etcd *etcdRunner) StopETCD() {
	if etcd.etcdCommand != nil {
		etcd.etcdCommand.Process.Signal(syscall.SIGINT)
		etcd.etcdCommand.Process.Wait()
		etcd.etcdCommand = nil
		os.Remove("/tmp/log")
		os.Remove("/tmp/info")
		os.Remove("/tmp/snapshot")
		os.Remove("/tmp/conf")
	}
}

func (etcd *etcdRunner) exists() bool {
	_, err := os.Stat("/tmp/info")
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}
