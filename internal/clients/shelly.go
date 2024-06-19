package clients

import (
	"home_automation/internal/logger"
	"home_automation/internal/models"
	"home_automation/internal/utils"
	"strings"
	"time"

	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/dpt"
)

type ShellyClient struct {
	KnxClient *KnxClient
}

func InitShelly(config utils.Config, knxClient *KnxClient) *ShellyClient {

	for _, deviceConfig := range config.Shelly.ShellyDevices {
		device, err := deviceConfig.ToShellyDevice()
		if err != nil {
			logger.Warning("Failed creating shelly device %s from config: %s\n", deviceConfig.Ip, err)
			continue
		}
		utils.KnxShellyMap[deviceConfig.KnxAddress] = device
	}
	return &ShellyClient{KnxClient: knxClient}
}

func (shellyClient *ShellyClient) HandleKnxMessage(knxAddr string, msg knx.GroupEvent) {
	shellyDevice := utils.KnxShellyMap[knxAddr]
	logger.Debug("Handlig shelly message for %+v", msg)
	if shellyDevice.Type == models.Relais {
		var relaisStateToSet dpt.DPT_1001
		relaisStateToSet.Unpack(msg.Data)
		relaisState, err := shellyDevice.SetRelaisValue(bool(relaisStateToSet))
		if err != nil {
			logger.Error("Failed to set relais value on device %s (%s): %s\n", shellyDevice.Name, shellyDevice.Ip, err)
			return
		}
		err = shellyClient.KnxClient.SendMessageToKnx(shellyDevice.KnxReturnAddress, dpt.DPT_1001(relaisState == 1).Pack())
		if err != nil {
			logger.Error("Warning: failed to send relais value back on KNX, but relais state (%d) set on shelly device!\n", relaisState)
		}
	}
}

func (shellyClient *ShellyClient) HandleFullStatusMessageMessage(message *models.ShellyFullStatusUpdate) error {
	// Check what source it is
	// currently only shelly H&T is supported
	if strings.HasPrefix(message.Source, "shelly") {
		// TODO: Implement
		logger.Trace("well ok we've recieved a shelly message")
	}
	return nil
}

func (shellyClient *ShellyClient) StartFetchShellyData(gauges utils.PromExporterGauges) {
	go func() {
		// Periodically fetch data for all shellies
		for range time.Tick(time.Second * 5) {
			logger.Trace("Getting status for all shelly devices")
			for knxAddr, shellyDevice := range utils.KnxShellyMap {
				switchStatusResponse, err := shellyDevice.GetStatus()
				if err != nil {
					logger.Warning("Failed getting status from shelly, skipping device %s", shellyDevice.Name)
					continue
				}
				switchStatus := switchStatusResponse.Switches[0]
				gauges.CurrentGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*switchStatus.Current)
				gauges.VoltageGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*switchStatus.Voltage)
				gauges.PowerConsumptionGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*switchStatus.APower)
				gauges.WifiSignalGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*switchStatusResponse.Wifi.RRSI)
				gauges.ShellyTempGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*switchStatus.Temperature.C)
			}
		}
	}()
}
