package interfaces

import (
	"fmt"
	"log"
	"os"

	"home_automation/internal/models"
	"home_automation/internal/utils"

	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/cemi"
	"github.com/vapourismo/knx-go/knx/dpt"
	"github.com/vapourismo/knx-go/knx/util"
)

type KnxClient = knx.GroupTunnel

var knxDevices = map[string]*models.KnxDevice{}
var knxClient KnxClient

func InitKnx(config utils.Config) *KnxClient {
	for _, deviceConfig := range config.Knx.KnxDevices {
		device, err := deviceConfig.ToKnxDevice()
		if err != nil {
			fmt.Printf("Failed creating knxDevice %s from config: %s\n", deviceConfig.KnxAddress, err)
			return nil
		}
		knxDevices[deviceConfig.KnxAddress] = device
	}

	for knxAddr, theShellyInfo := range KnxShellyMap {
		knxDevices[knxAddr] = &models.KnxDevice{Type: models.Actor, Name: theShellyInfo.Name, Room: theShellyInfo.Room, ValueType: models.Shelly}
	}

	// Setup logger for auxiliary logging. This enables us to see log messages from internal
	// routines.
	util.Logger = log.New(os.Stdout, "", log.LstdFlags)

	// Connect to the gateway.
	knxConnectionAddr := fmt.Sprintf("%s:%d", config.Knx.InterfaceIP, config.Knx.InterfacePort)
	var err error
	knxClient, err = knx.NewGroupTunnel(knxConnectionAddr, knx.DefaultTunnelConfig)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	return &knxClient
}

func ProcessKNXMessage(msg knx.GroupEvent, gauges utils.PromGauges) {
	// Map the destinations adressess to the corresponding types
	var temp dpt.DPT_9001
	var windspeed dpt.DPT_9005
	var lux dpt.DPT_9004
	var indicator dpt.DPT_1002
	var lightValue dpt.DPT_5001
	dest := msg.Destination.String()
	util.Logger.Printf("%+v", msg)
	if knxDevice, found := knxDevices[dest]; found {
		switch knxDevice.ValueType {
		case models.Temperatur:
			err := temp.Unpack(msg.Data)
			if err == nil {
				util.Logger.Printf("Temp: %+v: %v", msg, temp)
				gauges.TempGauge.WithLabelValues(dest, knxDevice.Room, knxDevice.Name).Set(float64(temp))
			} else {
				util.Logger.Printf("Failed to unpack temp for %s: %v", msg.Destination, err)
			}
		case models.Windspeed:
			err := windspeed.Unpack(msg.Data)
			if err == nil {
				util.Logger.Printf("Speed: %+v: %v", msg, windspeed)
				gauges.WindspeedGauge.Set(float64(windspeed))
			} else {
				util.Logger.Printf("Failed to unpack windspeed for %s: %v", msg.Destination, err)
			}
		case models.Brightness:
			err := lux.Unpack(msg.Data)
			if err == nil {
				util.Logger.Printf("Lux: %+v: %v", msg, lux)
				gauges.LuxGauge.Set(float64(lux))
			} else {
				util.Logger.Printf("Failed to unpack lux for %s: %v", msg.Destination, err)
			}
		case models.Indicator:
			err := indicator.Unpack(msg.Data)
			if err == nil {
				util.Logger.Printf("Indicator: %+v: %v", msg, indicator)
				if knxDevice.Name == "rain" {
					// It's raining
					if indicator == true {
						gauges.RainIndicator.Set(1)
					} else {
						gauges.RainIndicator.Set(0)
					}
				}
			} else {
				util.Logger.Printf("Failed to unpack indicator for %s: %v", msg.Destination, err)
			}
		case models.Shelly:
			if knxDevice.Type == models.Actor {
				ShellyHandleKnxMessage(dest, msg)
			} else {
				util.Logger.Printf("%s not a actor, ignoring message", knxDevice.Name)
			}
		case models.Light:
			err := lightValue.Unpack(msg.Data)
			if err == nil {
				util.Logger.Printf("Ligh: %+v: %v", msg, lightValue)
			} else {
				util.Logger.Printf("Failed to unpack lightValue for %s: %v", msg.Destination, err)
			}
		default:
			util.Logger.Printf("No type map for destination: %s", msg.Destination)
			util.Logger.Printf("%+v", msg)

		}
	} else {
		util.Logger.Printf("Destination %s not in destInfo map", msg.Destination)
	}
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
		util.Logger.Printf("Failed to send message (%v) with destination %s to the KNX bus: %s", data, destination, err)
		return err
	}

	return nil
}
