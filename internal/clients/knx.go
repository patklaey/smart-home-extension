package clients

import (
	"home_automation/internal/logger"

	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/cemi"
	"github.com/vapourismo/knx-go/knx/util"
)

type KnxClient struct {
	KnxTunnel knx.GroupTunnel
}

func InitKnxClient(knxTunnel knx.GroupTunnel) *KnxClient {
	return &KnxClient{
		KnxTunnel: knxTunnel,
	}
}

func (client *KnxClient) SendMessageToKnx(destination string, data []byte) error {

	cemiDesination, err := cemi.NewGroupAddrString(destination)
	if err != nil {
		util.Logger.Printf("Failed to convert destination to cemi address: %s", err)
		return err
	}
	err = client.KnxTunnel.Send(knx.GroupEvent{
		Command:     knx.GroupWrite,
		Destination: cemiDesination,
		Data:        data,
	})
	if err != nil {
		logger.Error("Failed to send message (%v) with destination %s to the KNX bus: %s", data, destination, err)
		return err
	}

	return nil
}
