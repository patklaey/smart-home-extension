package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"home_automation/internal/interfaces"
	"home_automation/internal/logger"
	"home_automation/internal/models"
	"home_automation/internal/utils"
	"net/http"
	"strings"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/dpt"
)

type ShellyClient struct {
	knxClient     interfaces.KnxClientInterface
	promGauges    utils.PromExporterGauges
	knxDevices    map[string]*models.KnxDevice
	shellyDevices map[string]*models.ShellyDevice // was a package-level global
	knxShellyMap  map[string]*models.ShellyDevice // was utils.KnxShellyMap global
}

func InitShelly(config *utils.Config, knxClient interfaces.KnxClientInterface, gauges utils.PromExporterGauges, devices map[string]*models.KnxDevice) *ShellyClient {
	shellyMap := make(map[string]*models.ShellyDevice)
	for _, deviceConfig := range config.Shelly.ShellyDevices {
		device, err := deviceConfig.ToShellyDevice()
		if err != nil {
			logger.Warning("Failed creating shelly device %s from config: %s\n", deviceConfig.Ip, err)
			continue
		}
		shellyMap[deviceConfig.KnxAddress] = device
		devices[device.KnxAddress] = &models.KnxDevice{
			Type:      models.Actor,
			Name:      device.Name,
			Room:      device.Room,
			ValueType: models.Shelly,
		}
	}
	return &ShellyClient{
		knxClient:     knxClient,
		promGauges:    gauges,
		knxDevices:    devices,
		shellyDevices: make(map[string]*models.ShellyDevice),
		knxShellyMap:  shellyMap,
	}
}

func (shellyClient *ShellyClient) HandleKnxMessage(knxAddr string, msg knx.GroupEvent) {
	// Use injected shellyMap instead of utils.KnxShellyMap global
	shellyDevice, found := shellyClient.knxShellyMap[knxAddr]
	if !found {
		logger.Warning("No shelly device found for KNX address %s, ignoring message", knxAddr)
		return
	}
	logger.Debug("Handling shelly message for %+v", msg)
	if shellyDevice.Type == models.Relais {
		var relaisStateToSet dpt.DPT_1001
		relaisStateToSet.Unpack(msg.Data)
		relaisState, err := shellyClient.SetRelaisValue(shellyDevice, bool(relaisStateToSet))
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
	var signal *float64
	var voltage *float64
	var apower *float64
	var current *float64
	var device *models.ShellyDevice

	switch {
	case strings.HasPrefix(message.Source, "shellyhtg3"):
		logger.Trace("According to device source (%s) it's a shelly H&T gen3 message", message.Source)
		return shellyClient.handleHTStatusUpdate(message)
	case strings.HasPrefix(message.Source, "shellypmminig3"):
		device = shellyClient.getShellyDeviceBySource(message.Source, *message.Parameters.Wifi.StaIP)
		if device == nil {
			logger.Warning("Device for source '%s' not found (not in config?), skipping.", message.Source)
			return nil
		}
		logger.Trace("According to device source (%s) it's a shelly PM1 mini gen3 message", message.Source)
		signal = message.Parameters.Wifi.RRSI
		voltage = message.Parameters.PM1.Voltage
		apower = message.Parameters.PM1.Apower
		current = message.Parameters.PM1.Current
	case strings.HasPrefix(message.Source, "shellyplus1pm") || strings.HasPrefix(message.Source, "shelly1pmminig3"):
		device = shellyClient.getShellyDeviceBySource(message.Source, *message.Parameters.Wifi.StaIP)
		if device == nil {
			logger.Warning("Device for source '%s' not found (not in config?), skipping.", message.Source)
			return nil
		}
		logger.Trace("According to device source (%s) it's a shelly relais message", message.Source)
		signal = message.Parameters.Wifi.RRSI
		voltage = message.Parameters.Switch.Voltage
		apower = message.Parameters.Switch.APower
		current = message.Parameters.Switch.Current
	default:
		logger.Trace("Unknown message from source %s, ignoring message", message.Source)
		return nil
	}

	shellyClient.promGauges.WifiSignalGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(*signal)
	shellyClient.promGauges.VoltageGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(*voltage)
	shellyClient.promGauges.CurrentGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(*current)
	shellyClient.promGauges.PowerConsumptionGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(*apower)
	return nil
}

func (shellyClient *ShellyClient) handleHTStatusUpdate(message *models.ShellyStatusUpdate) error {
	var lastError error
	for knxAddress, device := range shellyClient.knxDevices {
		if device.Name != "Shelly H&T" {
			continue
		}
		switch device.ValueType {
		case models.Temperatur:
			logger.Debug("Found shelly h&t temperature device with knxAddress: %s", knxAddress)
			temperature := message.Parameters.Temperatures.TC
			shellyClient.promGauges.TempGauge.WithLabelValues(knxAddress, device.Room, device.Name).Set(temperature)
			if err := shellyClient.knxClient.SendMessageToKnx(knxAddress, dpt.DPT_9001(temperature).Pack()); err != nil {
				logger.Error("Warning: failed to send temperature value (%.2f) to KNX", temperature)
				lastError = err
			} else {
				logger.Debug("Successfully sent temperature value (%.2f) to KNX", temperature)
			}
		case models.Humidity:
			logger.Debug("Found shelly h&t humidity device with knxAddress: %s", knxAddress)
			humidity := message.Parameters.Humidities.Humidity
			shellyClient.promGauges.HumidityGauge.WithLabelValues(knxAddress, device.Room, device.Name).Set(humidity)
			// Even though DPT_9007 would be correct, iBricks does not work with that, so using DPT_9001 for humidity too
			if err := shellyClient.knxClient.SendMessageToKnx(knxAddress, dpt.DPT_9001(humidity).Pack()); err != nil {
				logger.Error("Warning: failed to send humidity value (%.2f) to KNX", humidity)
				lastError = err
			} else {
				logger.Debug("Successfully sent humidity value (%.2f) to KNX", humidity)
			}
		}
	}
	return lastError
}

func (shellyClient *ShellyClient) StartFetchShellyData(ctx context.Context, gauges utils.PromExporterGauges, frequency int) {
	go func() {
		ticker := time.NewTicker(time.Second * time.Duration(frequency))
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				logger.Trace("Getting status for all shelly devices")
				// Use injected shellyMap instead of utils.KnxShellyMap global
				for knxAddr, shellyDevice := range shellyClient.knxShellyMap {
					shellyStatusResponse, err := shellyClient.GetStatus(shellyDevice)
					if err != nil {
						logger.Warning("Failed getting status from shelly, skipping device %s", shellyDevice.Name)
						continue
					}
					switch shellyDevice.Type {
					case models.Meter:
						// nothing to do yet
					case models.Relais:
						temp := *shellyStatusResponse.Switch.Temperature.C
						gauges.ShellyTempGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(temp)
					default:
						logger.Warning("Unknown shelly device type '%d', skipping device '%s'", shellyDevice.Type, shellyDevice.Name)
					}
					gauges.WifiSignalGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*shellyStatusResponse.Wifi.RRSI)
				}
				logger.Trace("Done fetching status for all shellies")
			case <-ctx.Done():
				logger.Info("Stopping shelly data fetching routine")
				return
			}
		}
	}()
}

func (shellyClient *ShellyClient) HandleWebSocketMessage(messageContent []byte) error {
	var shellyMessage *models.ShellyStatusUpdate
	if err := json.Unmarshal(messageContent, &shellyMessage); err != nil {
		logger.Error("Could not unmarshall message to map: %s", err)
		return err
	}

	switch shellyMessage.Method {
	case models.ShellyNotifyFullStatus:
		if err := shellyClient.HandleFullStatusMessageMessage(shellyMessage); err != nil {
			logger.Warning("The following message received on the websocket could not successfully be handled by the shelly client: %s", string(messageContent))
			return err
		}
		logger.Trace("%s message successfully processed", models.ShellyNotifyFullStatus)
	case models.ShellyNotifStatus:
		if err := shellyClient.HandleStatusMessage(shellyMessage); err != nil {
			logger.Warning("The following message received on the websocket could not successfully be handled by the shelly client: %s", string(messageContent))
			return err
		}
		logger.Trace("%s message successfully processed", models.ShellyNotifStatus)
	default:
		logger.Warning("Unexpected method from shelly websocket message received: '%s'", shellyMessage.Method)
	}
	return nil
}

func (shellyClient *ShellyClient) HandleStatusMessage(message *models.ShellyStatusUpdate) error {
	device, found := shellyClient.shellyDevices[message.Source]
	if !found {
		logger.Info("Shelly device '%s' not yet known, need to wait for next full status update", message.Source)
		return nil
	}

	var voltage, apower, current *float64
	if message.Parameters.PM1 != nil {
		voltage = message.Parameters.PM1.Voltage
		apower = message.Parameters.PM1.Apower
		current = message.Parameters.PM1.Current
	}
	if message.Parameters.Switch != nil {
		voltage = message.Parameters.Switch.Voltage
		apower = message.Parameters.Switch.APower
		current = message.Parameters.Switch.Current
	}
	if voltage != nil {
		shellyClient.promGauges.VoltageGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(*voltage)
	}
	if current != nil {
		shellyClient.promGauges.CurrentGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(*current)
	}
	if apower != nil {
		shellyClient.promGauges.PowerConsumptionGauge.WithLabelValues(device.KnxAddress, device.Room, device.Name, device.Ip).Set(*apower)
	}
	return nil
}

// getShellyDeviceBySource looks up a ShellyDevice by its source identifier,
// caching the result in shellyDevices for subsequent status updates.
func (shellyClient *ShellyClient) getShellyDeviceBySource(source string, deviceIp string) *models.ShellyDevice {
	if device, found := shellyClient.shellyDevices[source]; found {
		return device
	}
	for _, knxShellyDevice := range shellyClient.knxShellyMap {
		if knxShellyDevice.Ip == deviceIp {
			shellyClient.shellyDevices[source] = knxShellyDevice
			return knxShellyDevice
		}
	}
	return nil
}

func (client *ShellyClient) GetStatus(actor *models.ShellyDevice) (*models.ShellyGetStatusResponse, error) {
	var response models.ShellyGetStatusResponse
	logger.Trace("Get status for shelly device %s", actor.Name)
	requestUrl := fmt.Sprintf("http://%s/rpc/Shelly.GetStatus", actor.Ip)

	// Create a client with a short timeout in case some devices are not reachable
	httpClient := http.Client{Timeout: 5 * time.Second}

	err := requests.
		URL(requestUrl).
		Client(&httpClient).
		ToJSON(&response).
		Fetch(context.Background())

	if err != nil {
		logger.Error("Failed to get status for shelly device %s (%s): %s", actor.Name, actor.Ip, err)
		return nil, err
	}
	return &response, nil
}

func (client *ShellyClient) SetRelaisValue(actor *models.ShellyDevice, value bool) (int, error) {
	requestUrl := fmt.Sprintf("http://%s/relay/%d", actor.Ip, actor.Index)
	var response models.ShellyRelaisActionResponse
	reqBuilder := requests.URL(requestUrl).ToJSON(&response)
	if value == true {
		reqBuilder.Param("turn", "on")
	} else {
		reqBuilder.Param("turn", "off")
	}
	err := reqBuilder.Fetch(context.Background())
	if err != nil {
		logger.Error("Failed to set relais status for shelly device %s (%s): %s", actor.Name, actor.Ip, err)
		return -1, err
	}

	if value != response.IsOn {
		return -1, fmt.Errorf("response of the switch %s (%t) does not match requested state (%t)", actor.Name, response.IsOn, value)

	}
	return btoi(response.IsOn), nil
}

func btoi(boolean bool) int {
	if boolean {
		return 1
	}
	return 0
}
