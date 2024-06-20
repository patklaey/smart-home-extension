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
	knxClient  *KnxClient
	promGauges utils.PromExporterGauges
}

func InitShelly(config utils.Config, knxClient *KnxClient, gauges utils.PromExporterGauges) *ShellyClient {

	for _, deviceConfig := range config.Shelly.ShellyDevices {
		device, err := deviceConfig.ToShellyDevice()
		if err != nil {
			logger.Warning("Failed creating shelly device %s from config: %s\n", deviceConfig.Ip, err)
			continue
		}
		utils.KnxShellyMap[deviceConfig.KnxAddress] = device
	}
	return &ShellyClient{knxClient: knxClient, promGauges: gauges}
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
		err = shellyClient.knxClient.SendMessageToKnx(shellyDevice.KnxReturnAddress, dpt.DPT_1001(relaisState == 1).Pack())
		if err != nil {
			logger.Error("Warning: failed to send relais value back on KNX, but relais state (%d) set on shelly device!\n", relaisState)
		}
	}
}

func (shellyClient *ShellyClient) HandleFullStatusMessageMessage(message *models.ShellyFullStatusUpdate) error {
	// Check what source it is
	// currently only shelly H&T is supported
	var lastError error
	lastError = nil
	if strings.HasPrefix(message.Source, "shellyhtg3") {
		for knxAddress, device := range utils.KnxDevices {
			if device.Name == "Shelly H&T" {
				if device.ValueType == models.Temperatur {
					logger.Debug("Found shelly h&t temperature device with knxAdress: %s", knxAddress)
					temperature := message.Parameters.Temperatures.TC
					shellyClient.promGauges.TempGauge.WithLabelValues(knxAddress, device.Room, device.Name).Set(temperature)
					err := shellyClient.knxClient.SendMessageToKnx(knxAddress, dpt.DPT_9001(temperature).Pack())
					if err != nil {
						logger.Error("Warning: failed to send temperature value (%.2f) to KNX", temperature)
						lastError = err
					} else {
						logger.Debug("Successfully sent temperature value (%.2f) to KNX", temperature)
					}
					continue
				}
				if device.ValueType == models.Humidity {
					logger.Debug("Found shelly h&t humidity device with knxAdress: %s", knxAddress)
					humidity := message.Parameters.Humidities.Humidity
					shellyClient.promGauges.HumidityGauge.WithLabelValues(knxAddress, device.Room, device.Name).Set(humidity)
					// Even though DPT_9007 would be correct, iBricks does not work with that therefore using also for
					// the humidity DPT_9001
					err := shellyClient.knxClient.SendMessageToKnx(knxAddress, dpt.DPT_9001(humidity).Pack())
					if err != nil {
						logger.Error("Warning: failed to send humidity value (%.2f) to KNX", humidity)
						lastError = err
					} else {
						logger.Debug("Successfully sent humidity value (%.2f) to KNX", humidity)
					}
					continue
				}
			}
		}
	} else {
		logger.Trace("Unknown message from source %s, ignoring message", message.Source)
	}
	return lastError
}

func (shellyClient *ShellyClient) StartFetchShellyData(gauges utils.PromExporterGauges) {
	go func() {
		// Periodically fetch data for all shellies
		for range time.Tick(time.Second * 5) {
			logger.Trace("Getting status for all shelly devices")
			for knxAddr, shellyDevice := range utils.KnxShellyMap {
				shellyStatusResponse, err := shellyDevice.GetStatus()
				if err != nil {
					logger.Warning("Failed getting status from shelly, skipping device %s", shellyDevice.Name)
					continue
				}
				var current, voltage, apower, temp float64
				switch shellyDevice.Type {
				case models.Meter:
					current = shellyStatusResponse.PM1.Current
					voltage = shellyStatusResponse.PM1.Voltage
					apower = shellyStatusResponse.PM1.Apower
				case models.Relais:
					current = *shellyStatusResponse.Switch.Current
					voltage = *shellyStatusResponse.Switch.Voltage
					apower = *shellyStatusResponse.Switch.APower
					temp = *shellyStatusResponse.Switch.Temperature.C
				default:
					logger.Warning("Unknown shelly device type '%d', skipping device '%s'", shellyDevice.Type, shellyDevice.Name)
				}
				gauges.CurrentGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(current)
				gauges.VoltageGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(voltage)
				gauges.PowerConsumptionGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(apower)
				gauges.WifiSignalGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*shellyStatusResponse.Wifi.RRSI)
				if shellyDevice.Type == models.Relais {
					gauges.ShellyTempGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(temp)
				}
			}
			logger.Trace("Done fetching status for all shellies")
		}
	}()
}
