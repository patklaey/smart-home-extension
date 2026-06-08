package models

import (
	goShelly "github.com/jcodybaker/go-shelly"
)

const (
	ShellyNotifyFullStatus = "NotifyFullStatus"
	ShellyNotifStatus      = "NotifyStatus"
	ShellyNotifEvent       = "NotifyEvent"
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

type EM1 struct {
	ActPower    *float64 `json:"act_power,omitempty"`
	Calibration string   `json:"calibration,omitempty"`
	Current     *float64 `json:"current,omitempty"`
	Freq        *float64 `json:"freq,omitempty"`
	Id          int      `json:"id"`
	Voltage     *float64 `json:"voltage,omitempty"`
}

type EM1Data struct {
	Id                int     `json:"id"`
	TotalActEnergy    float64 `json:"total_act_energy,omitempty"`
	TotalActRetEnergy float64 `json:"total_act_ret_energy,omitempty"`
}

type MatterStatus struct {
	Comissionable bool `json:"commissionable,omitempty"`
	NumFabrics    int  `json:"num_fabrics,omitempty"`
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
	Timestamp    float64                      `json:"ts"`
	BLE          *goShelly.BLEStatus          `json:"ble,omitempty"`
	BtHome       *goShelly.BTHomeDeviceStatus `json:"bthome,omitempty"`
	Cloud        *goShelly.CloudStatus        `json:"cloud,omitempty"`
	PM1          *PM1                         `json:"pm1:0,omitempty"`
	EM1          *EM1                         `json:"em1:0,omitempty"`
	EM1Data      *EM1Data                     `json:"em1data:0,omitempty"`
	Matter       *MatterStatus                `json:"matter,omitempty"`
	MQTT         *goShelly.MQTTStatus         `json:"mqtt,omitempty"`
	System       *goShelly.SysStatus          `json:"sys,omitempty"`
	Wifi         *goShelly.WifiStatus         `json:"wifi,omitempty"`
	Switch       *goShelly.SwitchStatus       `json:"switch:0,omitempty"`
	DevicePowers ShellyDevicePower            `json:"devicepower:0,omitempty"`
	Websocket    ShellyWebsocketStatus        `json:"ws,omitempty"`
	Humidities   ShellyHumidityStatus         `json:"humidity:0,omitempty"`
	Temperatures ShellyTemperatureStatus      `json:"temperature:0,omitempty"`
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
