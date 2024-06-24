package clients

import (
	"encoding/json"
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

var shellyDevices map[string]*models.ShellyDevice

func InitShelly(config *utils.Config, knxClient *KnxClient, gauges utils.PromExporterGauges) *ShellyClient {
	for _, deviceConfig := range config.Shelly.ShellyDevices {
		device, err := deviceConfig.ToShellyDevice()
		if err != nil {
			logger.Warning("Failed creating shelly device %s from config: %s\n", deviceConfig.Ip, err)
			continue
		}
		utils.KnxShellyMap[deviceConfig.KnxAddress] = device
		utils.KnxDevices[device.KnxAddress] = &models.KnxDevice{Type: models.Actor, Name: device.Name, Room: device.Room, ValueType: models.Shelly}
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

func (shellyClient *ShellyClient) HandleFullStatusMessageMessage(message *models.ShellyStatusUpdate) error {
	var lastError error
	lastError = nil
	if shellyDevices == nil {
		shellyDevices = map[string]*models.ShellyDevice{}
	}
	var device *models.ShellyDevice
	if d, found := shellyDevices[message.Source]; found {
		device = d
	} else {
		// Get device from knxShellyMap
		for _, knxShellyDevice := range utils.KnxShellyMap {
			if knxShellyDevice.Ip == *message.Parameters.Wifi.StaIP {
				device = knxShellyDevice
				shellyDevices[message.Source] = knxShellyDevice
				break
			}
		}
	}
	if device == nil {
		logger.Warning("Device for source '%s' not found (not in config?), skipping.", message.Source)
		return nil
	}
	// Check what source it is
	var signal float64
	var voltage float64
	var apower float64
	var current float64
	switch {
	case strings.HasPrefix(message.Source, "shellyhtg3"):
		logger.Trace("According to device source (%s) it's a shelly H&T gen3 message", message.Source)
		lastError = shellyClient.handleHTStatusUpdate(message, lastError)
		return lastError
	case strings.HasPrefix(message.Source, "shellypmminig3"):
		logger.Trace("According to device source (%s) it's a shelly PM1 mini gen3 message", message.Source)
		signal = *message.Parameters.Wifi.RRSI
		voltage = message.Parameters.PM1.Voltage
		apower = message.Parameters.PM1.Apower
		current = message.Parameters.PM1.Current
	case strings.HasPrefix(message.Source, "shellyplus1pm") || strings.HasPrefix(message.Source, "shelly1pmminig3"):
		logger.Trace("According to device source (%s) it's a shelly relais message", message.Source)
		signal = *message.Parameters.Wifi.RRSI
		voltage = *message.Parameters.Switch.Voltage
		apower = *message.Parameters.Switch.APower
		current = *message.Parameters.Switch.Current
	default:
		logger.Trace("Unknown message from source %s, ignoring message", message.Source)
	}

	// Set all gauges accordingly
	shellyClient.promGauges.WifiSignalGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(signal)
	shellyClient.promGauges.VoltageGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(voltage)
	shellyClient.promGauges.CurrentGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(current)
	shellyClient.promGauges.PowerConsumptionGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(apower)
	return lastError
}

func (shellyClient *ShellyClient) handleHTStatusUpdate(message *models.ShellyStatusUpdate, lastError error) error {
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
	return lastError
}

func (shellyClient *ShellyClient) StartFetchShellyData(gauges utils.PromExporterGauges, frequency int) {
	go func() {
		// Periodically fetch data for all shellies
		for range time.Tick(time.Second * time.Duration(frequency)) {
			logger.Trace("Getting status for all shelly devices")
			for knxAddr, shellyDevice := range utils.KnxShellyMap {
				shellyStatusResponse, err := shellyDevice.GetStatus()
				if err != nil {
					logger.Warning("Failed getting status from shelly, skipping device %s", shellyDevice.Name)
					continue
				}
				var temp float64
				switch shellyDevice.Type {
				case models.Meter:

				case models.Relais:
					temp = *shellyStatusResponse.Switch.Temperature.C
					gauges.ShellyTempGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(temp)
				default:
					logger.Warning("Unknown shelly device type '%d', skipping device '%s'", shellyDevice.Type, shellyDevice.Name)
				}

				gauges.WifiSignalGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*shellyStatusResponse.Wifi.RRSI)

			}
			logger.Trace("Done fetching status for all shellies")
		}
	}()
}

func (shellyClient *ShellyClient) HandleWebSocketMessage(messageContent []byte) error {

	var shellyMessage *models.ShellyStatusUpdate
	err := json.Unmarshal(messageContent, &shellyMessage)
	if err != nil {
		logger.Error("Could not unmarshall message to map: %s", err)
		return err
	}

	switch shellyMessage.Method {
	case models.ShellyNotifyFullStatus:
		err = shellyClient.HandleFullStatusMessageMessage(shellyMessage)
		if err != nil {
			logger.Warning("The following message received on the websocket could not successfully be handled by the shelly client: %s", string(messageContent))
			return err
		} else {
			logger.Trace("%s message successfully processed", models.ShellyNotifyFullStatus)
			return nil
		}
	case models.ShellyNotifStatus:
		err = shellyClient.HandleStatusMessage(shellyMessage)
		if err != nil {
			logger.Warning("The following message received on the websocket could not successfully be handled by the shelly client: %s", string(messageContent))
			return err
		} else {
			logger.Trace("%s message successfully processed", models.ShellyNotifStatus)
			return nil
		}
	default:
		logger.Error("Unexpected method from shelly websocket message received: '%s'", shellyMessage.Method)
	}
	return nil
}

func (shellyClient *ShellyClient) HandleStatusMessage(message *models.ShellyStatusUpdate) error {
	// Only proceed if the device is already known
	if device, found := shellyDevices[message.Source]; found {
		// As it's not known what data is sent, we need to test for all options
		if message.Parameters.PM1 != nil && message.Parameters.PM1.Voltage != 0 {
			voltage := message.Parameters.PM1.Voltage
			apower := message.Parameters.PM1.Apower
			current := message.Parameters.PM1.Current
			shellyClient.promGauges.VoltageGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(voltage)
			shellyClient.promGauges.CurrentGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(current)
			shellyClient.promGauges.PowerConsumptionGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(apower)
		}
	} else {
		logger.Info("Shelly device '%s' not yet known, need to wait for next full status update", message.Source)
	}
	return nil
}
