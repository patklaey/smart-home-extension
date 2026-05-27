package models

import (
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
type ShellyRelaisActionResponse struct {
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
