package interfaces

import (
	"fmt"
	"log"
	"os"
	"time"

	"home_automation/internal/logger"
	"home_automation/internal/models"
	"home_automation/internal/utils"

	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/cemi"
	"github.com/vapourismo/knx-go/knx/dpt"
	"github.com/vapourismo/knx-go/knx/util"
)

var (
	windShutterUpLow  float64
	windShutterUpMed  float64
	windShutterUpHigh float64

	windShutterUpLowActive  = true
	windShutterUpMedActive  = true
	windShutterUpHighActive = true
)

type KnxClient = knx.GroupTunnel

var knxDevices = map[string]*models.KnxDevice{}
var knxClient KnxClient

func InitKnx(config utils.Config) *KnxClient {
	for _, deviceConfig := range config.Knx.KnxDevices {
		device, err := deviceConfig.ToKnxDevice()
		if err != nil {
			logger.Error("Failed creating knxDevice %s from config: %s\n", deviceConfig.KnxAddress, err)
			return nil
		}
		knxDevices[deviceConfig.KnxAddress] = device
	}

	for knxAddr, theShellyInfo := range KnxShellyMap {
		knxDevices[knxAddr] = &models.KnxDevice{Type: models.Actor, Name: theShellyInfo.Name, Room: theShellyInfo.Room, ValueType: models.Shelly}
	}

	// Setup windspeed thresholds
	windShutterUpHigh = config.Weather.Windspeed.ShutteUpHigh
	windShutterUpMed = config.Weather.Windspeed.ShutteUpMed
	windShutterUpLow = config.Weather.Windspeed.ShutteUpLow

	// Setup logger for auxiliary logging. This enables us to see log messages from internal
	// routines.
	util.Logger = log.New(os.Stdout, "", log.LstdFlags)

	// Connect to the gateway.
	knxConnectionAddr := fmt.Sprintf("%s:%d", config.Knx.InterfaceIP, config.Knx.InterfacePort)
	var err error
	knxClient, err = knx.NewGroupTunnel(knxConnectionAddr, knx.DefaultTunnelConfig)
	if err != nil {
		logger.Error(err.Error())
		return nil
	}

	return &knxClient
}

func ProcessKNXMessage(msg knx.GroupEvent, gauges utils.PromExporterGauges) {
	// Map the destinations adressess to the corresponding types
	var temp dpt.DPT_9001
	var windspeed dpt.DPT_9005
	var lux dpt.DPT_9004
	var indicator dpt.DPT_1002
	var lightValue dpt.DPT_5001
	dest := msg.Destination.String()
	logger.Trace("%+v", msg)
	if knxDevice, found := knxDevices[dest]; found {
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
				checkShutterUp(float64(windspeed))
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
				ShellyHandleKnxMessage(dest, msg)
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
		logger.Debug("Destination %s not in destInfo map", msg.Destination)
	}
}

func checkShutterUp(windspeed float64) {
	switch {
	case windspeed >= windShutterUpHigh:
		if windShutterUpHighActive {
			err := shutterUp(models.WindClass{}.High())
			if err == nil {
				windShutterUpHighActive = false
				logger.Info("Shutters for high wind retracted")
				time.AfterFunc(15*time.Minute, func() { windShutterUpHighActive = true })
			} else {
				logger.Warning("Some or all shutters could not be retracted (trigger high wind)")
			}
		}
	case windspeed >= windShutterUpMed:
		if windShutterUpMedActive {
			err := shutterUp(models.WindClass{}.Medium())
			if err == nil {
				windShutterUpMedActive = false
				logger.Info("Shutters for medium wind retracted")
				time.AfterFunc(15*time.Minute, func() { windShutterUpMedActive = true })
			} else {
				logger.Warning("Some or all shutters could not be retracted (trigger medium wind)")
			}
		}
	case windspeed >= windShutterUpLow:
		if windShutterUpLowActive {
			err := shutterUp(models.WindClass{}.Low())
			if err == nil {
				windShutterUpLowActive = false
				logger.Info("Shutters for low wind retracted")
				time.AfterFunc(15*time.Minute, func() { windShutterUpLowActive = true })
			} else {
				logger.Warning("Some or all shutters could not be retracted (trigger low wind)")
			}
		}
	}
}

func shutterUp(windClass int) error {
	var lastError error
	lastError = nil
	for knxAddress, knxDevice := range knxDevices {
		if knxDevice.Type == models.Actor && knxDevice.ValueType == models.Shutter && knxDevice.ShutterDevice.WindClass <= windClass {
			err := SendMessageToKnx(knxAddress, dpt.DPT_1001(false).Pack())
			if err != nil {
				logger.Error("Failed to send shutterUp command for shutter %s (%s): %s\n", knxDevice.Name, knxAddress, err)
				lastError = err
			}
		}
	}
	return lastError
}

func SendMessageToKnx(destination string, data []byte) error {

	cemiDesination, err := cemi.NewGroupAddrString(destination)
	if err != nil {
		util.Logger.Printf("Failed to convert destination to cemi address: %s", err)
		return err
	}
	err = knxClient.Send(knx.GroupEvent{
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
