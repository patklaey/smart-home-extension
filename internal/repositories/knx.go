package repositories

import (
	"fmt"
	"log"
	"os"
	"time"

	"home_automation/internal/clients"
	"home_automation/internal/interfaces"
	"home_automation/internal/logger"
	"home_automation/internal/models"
	"home_automation/internal/utils"

	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/dpt"
	"github.com/vapourismo/knx-go/knx/util"
)

type KnxRepository struct {
	KnxTunnel  knx.GroupTunnel
	KnxClient  *clients.KnxClient
	KnxDevices map[string]*models.KnxDevice
}

func InitAndConnectKnx(config *utils.Config) *KnxRepository {
	devices := make(map[string]*models.KnxDevice)
	for _, deviceConfig := range config.Knx.KnxDevices {
		device, err := deviceConfig.ToKnxDevice()
		if err != nil {
			logger.Error("Failed creating knxDevice %s from config: %s\n", deviceConfig.KnxAddress, err)
			return nil
		}
		devices[deviceConfig.KnxAddress] = device
	}

	// Setup logger for auxiliary logging. This enables us to see log messages from internal
	// routines.
	util.Logger = log.New(os.Stdout, "", log.LstdFlags)

	// Connect to the gateway.
	knxConnectionAddr := fmt.Sprintf("%s:%d", config.Knx.InterfaceIP, config.Knx.InterfacePort)
	tunnel, err := knx.NewGroupTunnel(knxConnectionAddr, knx.DefaultTunnelConfig)
	if err != nil {
		logger.Error("Failed to connect to KNX tunnel: %v", err)
		return nil
	}

	return &KnxRepository{KnxTunnel: tunnel, KnxClient: &clients.KnxClient{KnxTunnel: tunnel}, KnxDevices: devices}
}

func (knxRepo *KnxRepository) ListenToKNX(gauges utils.PromExporterGauges, weatherMonitor interfaces.WeatherMonitorInterface, shellyClient interfaces.ShellyClientInterface) {
	go func() {
		// Receive messages from the gateway. The inbound channel is closed with the connection.
		for msg := range knxRepo.KnxTunnel.Inbound() {
			knxRepo.processKNXMessage(msg, gauges, weatherMonitor, shellyClient)
		}
	}()
}

func (knxRepo *KnxRepository) processKNXMessage(msg knx.GroupEvent, gauges utils.PromExporterGauges, weatherMonitor interfaces.WeatherMonitorInterface, shellyClient interfaces.ShellyClientInterface) {
	// Map the destinations adressess to the corresponding types
	var temp dpt.DPT_9001
	var windspeed dpt.DPT_9005
	var lux dpt.DPT_9004
	var indicator dpt.DPT_1002
	var lightValue dpt.DPT_5001
	dest := msg.Destination.String()
	logger.Trace("%+v", msg)
	if knxDevice, found := knxRepo.KnxDevices[dest]; found {
		switch knxDevice.ValueType {
		case models.Temperatur:
			err := temp.Unpack(msg.Data)
			if err == nil {
				logger.Debug("Temp: %+v: %v", msg, temp)
				gauges.TempGauge.WithLabelValues(dest, knxDevice.Room, knxDevice.Name).Set(float64(temp))
			} else {
				logger.Error("Failed to unpack temp for %s: %v", msg.Destination, err)
			}
		case models.Windspeed:
			err := windspeed.Unpack(msg.Data)
			if err == nil {
				logger.Debug("Speed: %+v: %v", msg, windspeed)
				weatherMonitor.CheckShutterUp(float64(windspeed))
				gauges.WindspeedGauge.Set(float64(windspeed))
			} else {
				logger.Error("Failed to unpack windspeed for %s: %v", msg.Destination, err)
			}
		case models.Brightness:
			err := lux.Unpack(msg.Data)
			if err == nil {
				logger.Debug("Lux: %+v: %v", msg, lux)
				gauges.LuxGauge.Set(float64(lux))
			} else {
				logger.Error("Failed to unpack lux for %s: %v", msg.Destination, err)
			}
		case models.Indicator:
			err := indicator.Unpack(msg.Data)
			if err == nil {
				logger.Debug("Indicator: %+v: %v", msg, indicator)
				if knxDevice.Name == "weatherstation" {
					// It's raining
					if indicator == true {
						gauges.RainIndicator.Set(1)
					} else {
						gauges.RainIndicator.Set(0)
					}
				}
			} else {
				logger.Error("Failed to unpack indicator for %s: %v", msg.Destination, err)
			}
		case models.Shelly:
			if knxDevice.Type == models.Actor {
				shellyClient.HandleKnxMessage(dest, msg)
			} else {
				logger.Warning("%s not a actor, ignoring message", knxDevice.Name)
			}
		case models.Light:
			err := lightValue.Unpack(msg.Data)
			if err == nil {
				logger.Debug("Ligh: %+v: %v", msg, lightValue)
			} else {
				logger.Error("Failed to unpack lightValue for %s: %v", msg.Destination, err)
			}
		default:
			logger.Warning("No type map for destination: %s", msg.Destination)
		}
	} else {
		logger.Trace("Destination %s not in destInfo map", msg.Destination)
	}
}

func (knxRepo *KnxRepository) MonitorKnxHealth(frequency int, healthStatus *utils.HealthStatus) {

	go func() {
		// regularly check if message can be sent to bus
		ticker := time.NewTicker(time.Minute * time.Duration(frequency))
		for ; true; <-ticker.C {
			err := knxRepo.KnxClient.SendMessageToKnx("1/0/5", []byte{})
			if err != nil {
				logger.Error("Failed to send message: %v. Setting health status to 0", err)
				healthStatus.SetKnxHealthStatus(0)
			} else {
				healthStatus.SetKnxHealthStatus(1)
			}
		}
	}()

}
