package models

import (
	"context"
	"fmt"

	"home_automation/internal/logger"

	"github.com/carlmjohnson/requests"
	goShelly "github.com/jcodybaker/go-shelly"
)

const (
	// Value Types
	Temperatur = iota
	Humidity
	Windspeed
	Brightness
	Relais
	Shutter
	Light
	Indicator
	Shelly
	Meter

	// Types
	Sensor
	Actor

	// Rooms
	LivingRoom    = "LivingRoom"
	Kitchen       = "Kitchen"
	Dining        = "Dining"
	OfficeSteffi  = "OfficeSteffi"
	OfficePat     = "OfficePat"
	BathroomSmall = "BathroomSmall"
	BathroomLarge = "BathroomLarge"
	Bedroom       = "Bedroom"
	Reduit        = "Reduit"
	Coridor       = "Coridor"
	Entry         = "Entry"
	Terrace       = "Terrace"
)

type KnxDevice struct {
	Type          int
	Name          string
	Room          string
	ValueType     int
	KnxAddress    string
	ShutterDevice ShutterDevice
}

type ShutterDevice struct {
	WindClass int
}

type WindClass struct{}

func (WindClass) Low() int {
	return 0
}

func (WindClass) Medium() int {
	return 1
}

func (WindClass) High() int {
	return 2
}

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
	BLE   *goShelly.BLEStatus   `json:"ble,omitempty"`
	Cloud *goShelly.CloudStatus `json:"cloud,omitempty"`
	MQTT  *goShelly.MQTTStatus  `json:"mqtt,omitempty"`
	PM1   struct {
		Id      int     `json:"id"`
		Voltage float64 `json:"voltage"`
		Current float64 `json:"current"`
		Apower  float64 `json:"apower"`
		Freq    float64 `json:"freq"`
		Aenergy struct {
			Total    float64   `json:"total"`
			ByMinute []float64 `json:"by_minute"`
			MinuteTs float64   `json:"minute_ts"`
		} `json:"aenergy"`
		RetAenergy struct {
			Total    float64   `json:"total"`
			ByMinute []float64 `json:"by_minute"`
			MinuteTs float64   `json:"minute_ts"`
		} `json:"ret_aenergy"`
	} `json:"pm1:0,omitempty"`
	System *goShelly.SysStatus    `json:"sys,omitempty"`
	Wifi   *goShelly.WifiStatus   `json:"wifi,omitempty"`
	Switch *goShelly.SwitchStatus `json:"switch:0,omitempty"`
	Ws     struct {
		Connected bool `json:"connected"`
	} `json:"ws,omitempty"`
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

type ShellyFullStatusUpdate struct {
	Source      string                            `json:"src"`
	Destination string                            `json:"dst"`
	Method      string                            `json:"method"`
	Parameters  *ShellyFullStatusUpdateParameters `json:"params"`
}

type ShellyFullStatusUpdateParameters struct {
	StatusUpdate *goShelly.ShellyGetStatusResponse `json:"inline"`
	Timestamp    float64                           `json:"ts"`
	DevicePowers ShellyDevicePower                 `json:"devicepower:0"`
	Websocket    ShellyWebsocketStatus             `json:"ws"`
	Humidities   ShellyHumidityStatus              `json:"humidity:0"`
	Temperatures ShellyTemperatureStatus           `json:"temperature:0"`
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

	err := requests.URL(requestUrl).
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
