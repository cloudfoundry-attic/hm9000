package cfcomponent

import (
	"errors"
	"fmt"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/yagnats"
)

type Config struct {
	Syslog                                 string
	VarzPort                               uint16
	VarzUser                               string
	VarzPass                               string
	NatsHosts                              []string
	NatsPort                               int
	NatsUser                               string
	NatsPass                               string
	MbusClient                             yagnats.NATSConn
	CollectorRegistrarIntervalMilliseconds int
}

var DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *Config) (natsClient yagnats.NATSConn, err error) {
	members := make([]string, 0)
	for _, natsHost := range c.NatsHosts {
		members = append(members, fmt.Sprintf("nats://%s:%s@%s:%d", c.NatsUser, c.NatsPass, natsHost, c.NatsPort))
	}

	natsClient, err = yagnats.Connect(members)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Could not connect to NATS: %v", err.Error()))
	}
	return natsClient, nil
}

func (c *Config) Validate(logger *gosteno.Logger) (err error) {
	c.MbusClient, err = DefaultYagnatsClientProvider(logger, c)
	return
}
