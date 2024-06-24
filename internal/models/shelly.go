package models

import (
	"context"
	"fmt"
	"home_automation/internal/logger"
	"net/http"
	"time"

	"github.com/carlmjohnson/requests"
	goShelly "github.com/jcodybaker/go-shelly"
)

const (
	ShellyNotifyFullStatus = "NotifyFullStatus"
	ShellyNotifStatus      = "NotifyStatus"
)

type ShellyDevice struct {
	Type             int
	Ip               string
	Name             string
	Room             string
	Index            int
	KnxAddress       string
	KnxReturnAddress string
}

type ShellyGetStatusResponse struct {
	BLE       *goShelly.BLEStatus    `json:"ble,omitempty"`
	Cloud     *goShelly.CloudStatus  `json:"cloud,omitempty"`
	MQTT      *goShelly.MQTTStatus   `json:"mqtt,omitempty"`
	PM1       *PM1                   `json:"pm1:0,omitempty"`
	System    *goShelly.SysStatus    `json:"sys,omitempty"`
	Wifi      *goShelly.WifiStatus   `json:"wifi,omitempty"`
	Switch    *goShelly.SwitchStatus `json:"switch:0,omitempty"`
	Websocket ShellyWebsocketStatus  `json:"ws,omitempty"`
}

type PM1 struct {
	Id         int                      `json:"id"`
	Voltage    *float64                 `json:"voltage,omitempty"`
	Current    *float64                 `json:"current,omitempty"`
	Apower     *float64                 `json:"apower,omitempty"`
	Freq       *float64                 `json:"freq,omitempty"`
	AEnergy    *goShelly.EnergyCounters `json:"aenergy,omitempty"`
	RetAEnergy *goShelly.EnergyCounters `json:"ret_aenergy,omitempty"`
}
type shellyRelaisActionResponse struct {
	IsOn           bool    `json:"ison"`
	HasTimer       bool    `json:"has_timer"`
	TimerStartedAt int     `json:"timer_started_at"`
	TimerDuration  float64 `json:"timer_duration"`
	TimerRemaining float64 `json:"timer_remaining"`
	Overpower      bool    `json:"overpower"`
	Source         string  `json:"source"`
}

type ShellyStatusUpdate struct {
	Source      string                        `json:"src"`
	Destination string                        `json:"dst"`
	Method      string                        `json:"method"`
	Parameters  *ShellyStatusUpdateParameters `json:"params"`
}

type ShellyStatusUpdateParameters struct {
	Timestamp    float64                 `json:"ts"`
	BLE          *goShelly.BLEStatus     `json:"ble,omitempty"`
	Cloud        *goShelly.CloudStatus   `json:"cloud,omitempty"`
	MQTT         *goShelly.MQTTStatus    `json:"mqtt,omitempty"`
	PM1          *PM1                    `json:"pm1:0,omitempty"`
	System       *goShelly.SysStatus     `json:"sys,omitempty"`
	Wifi         *goShelly.WifiStatus    `json:"wifi,omitempty"`
	Switch       *goShelly.SwitchStatus  `json:"switch:0,omitempty"`
	DevicePowers ShellyDevicePower       `json:"devicepower:0,omitempty"`
	Websocket    ShellyWebsocketStatus   `json:"ws,omitempty"`
	Humidities   ShellyHumidityStatus    `json:"humidity:0,omitempty"`
	Temperatures ShellyTemperatureStatus `json:"temperature:0,omitempty"`
}
type ShellyWebsocketStatus struct {
	Connected bool `json:"connected"`
}

type ShellyHumidityStatus struct {
	Id       int     `json:"id"`
	Humidity float64 `json:"rh"`
}

type ShellyTemperatureStatus struct {
	Id int     `json:"id"`
	TC float64 `json:"tC"`
	TF float64 `json:"tF"`
}

type ShellyDevicePower struct {
	Id      int `json:"id"`
	Battery struct {
		Voltage float64 `json:"V"`
		Percent int     `json:"percent"`
	} `json:"battery"`
	External struct {
		Present bool `json:"present"`
	} `json:"external"`
}

func (actor *ShellyDevice) GetStatus() (*ShellyGetStatusResponse, error) {
	var response ShellyGetStatusResponse
	logger.Trace("Get status for shelly device %s", actor.Name)
	requestUrl := fmt.Sprintf("http://%s/rpc/Shelly.GetStatus", actor.Ip)

	// Create a client with a short timeout in case some devices are not reachable
	httpClient := http.Client{Timeout: 3 * time.Second}

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

func (actor *ShellyDevice) SetRelaisValue(value bool) (int, error) {
	requestUrl := fmt.Sprintf("http://%s/relay/%d", actor.Ip, actor.Index)
	var response shellyRelaisActionResponse
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
