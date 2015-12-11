package legacycollectorregistrar

import (
	"encoding/json"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/hm9000/cfcomponent"
	"github.com/cloudfoundry/yagnats"
	"github.com/nats-io/nats"
)

type collectorRegistrar struct {
	*gosteno.Logger
	mBusClient yagnats.NATSConn
}

func NewCollectorRegistrar(mBusClient yagnats.NATSConn, logger *gosteno.Logger) *collectorRegistrar {
	return &collectorRegistrar{mBusClient: mBusClient, Logger: logger}
}

func (r collectorRegistrar) RegisterWithCollector(cfc cfcomponent.Component) error {
	r.Debugf("Registering component %s with collect at ip: %s, port: %d, username: %s, password %s", cfc.UUID, cfc.IpAddress, cfc.StatusPort, cfc.StatusCredentials[0], cfc.StatusCredentials[1])
	err := r.announceComponent(cfc)
	r.subscribeToComponentDiscover(cfc)

	return err
}

func (r collectorRegistrar) announceComponent(cfc cfcomponent.Component) error {
	json, err := json.Marshal(NewAnnounceComponentMessage(cfc))
	if err != nil {
		return err
	}
	r.mBusClient.Publish(AnnounceComponentMessageSubject, json)
	return nil
}

func (r collectorRegistrar) subscribeToComponentDiscover(cfc cfcomponent.Component) {
	var callback nats.MsgHandler
	callback = func(msg *nats.Msg) {
		json, err := json.Marshal(NewAnnounceComponentMessage(cfc))
		if err != nil {
			r.Warnf("Failed to marshal response to message [%s]: %s", DiscoverComponentMessageSubject, err.Error())
		}
		r.mBusClient.Publish(msg.Reply, json)
	}

	r.mBusClient.Subscribe(DiscoverComponentMessageSubject, callback)

	return
}
