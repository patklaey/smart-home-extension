package utils

import (
	"fmt"
	"home_automation/internal/models"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Weather       *WeatherConfig   `yaml:"weather"`
	Knx           *KnxConfig       `yaml:"knx"`
	Shelly        *ShellyConfig    `yaml:"shelly"`
	PromExporter  *PromExporter    `yaml:"promExporter"`
	LogLevel      string           `yaml:"logLevel"`
	IBricks       *IBricksConfig   `yaml:"iBricks"`
	Websocket     *WebsocketConfig `yaml:"websocket"`
	Ipgeolocation *Ipgeoloaction   `yaml:"ipgeolocation"`
}

type Ipgeoloaction struct {
	ApiKey         string `yaml:"apiKey"`
	FetchFrequency int    `yaml:"fetchFrequency"`
}

type WebsocketConfig struct {
	Path     string                   `yaml:"path"`
	Port     int                      `yaml:"port"`
	Upgrader *WebsocketUpgraderConfig `yaml:"upgrader"`
}

type WebsocketUpgraderConfig struct {
	ReadBufferSize  int `yaml:"readBufferSize"`
	WriteBufferSize int `yaml:"writeBufferSize"`
}

type IBricksConfig struct {
	URL                string `yaml:"url"`
	Port               int    `yaml:"port"`
	HeartbeatFrequency int    `yaml:"heartbeatFrequencyMin"`
}

type WeatherConfig struct {
	Windspeed *WindspeedConfig `yaml:"windspeed"`
}

type WindspeedConfig struct {
	ShutteUpLowThreshold  float64 `yaml:"shutterUpLowThreshold"`
	ShutteUpMedThreshold  float64 `yaml:"shutterUpMedThreshold"`
	ShutteUpHighThreshold float64 `yaml:"shutterUpHighThreshold"`
	CheckAverageFrequency int     `yaml:"checkAverageFrequencyMin"`
	WindResetGracePeriod  int     `yaml:"windResetGracePeriodMin"`
}

type KnxConfig struct {
	InterfaceIP   string            `yaml:"interfaceIp"`
	InterfacePort int               `yaml:"interfacePort"`
	KnxDevices    []KnxDeviceConfig `yaml:"knxDevices"`
}

type KnxDeviceConfig struct {
	DeviceBaseConfig `yaml:",inline"`
	ValueType        string      `yaml:"valueType"`
	TypeConfig       *TypeConfig `yaml:"typeConfig,omitempty"`
}

type TypeConfig struct {
	WindClass string `yaml:"windClass"`
}

type ShellyConfig struct {
	ShellyDevices              []ShellyDeviceConfig `yaml:"shellyDevices"`
	ShellyPullFrequencySeconds int                  `yaml:"pullFrequencySec"`
}

type ShellyDeviceConfig struct {
	DeviceBaseConfig `yaml:",inline"`
	Ip               string `yaml:"ip"`
	Index            int    `yaml:"index"`
	KnxReturnAddress string `yaml:"knxReturnAddress"`
}

type DeviceBaseConfig struct {
	KnxAddress string `yaml:"knxAddress"`
	Type       string `yaml:"type"`
	Name       string `yaml:"name"`
	Room       string `yaml:"room"`
}

type PromExporter struct {
	Port int    `yaml:"port"`
	Path string `yaml:"path"`
}

func LoadConfig(configFile string) *Config {
	var config Config

	yfile, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Println("Could not read config file: ", err)
		return nil
	}

	err = yaml.Unmarshal(yfile, &config)
	if err != nil {
		fmt.Println("Error loading configuration: ", err)
		return nil
	}
	return &config
}

func (deviceConfig *ShellyDeviceConfig) ToShellyDevice() (*models.ShellyDevice, error) {
	device := &models.ShellyDevice{
		Name:             deviceConfig.Name,
		Ip:               deviceConfig.Ip,
		Index:            deviceConfig.Index,
		KnxAddress:       deviceConfig.KnxAddress,
		KnxReturnAddress: deviceConfig.KnxReturnAddress,
	}

	switch strings.ToLower(deviceConfig.Type) {
	case "relais":
		device.Type = models.Relais
	case "meter":
		device.Type = models.Meter
	}

	room := getRoomFromString(deviceConfig.Room)
	if room == "" {
		return nil, fmt.Errorf("unknown KnxDevice room '%s'", deviceConfig.Room)
	}
	device.Room = room

	return device, nil
}

func (deviceConfig *KnxDeviceConfig) ToKnxDevice() (*models.KnxDevice, error) {
	device := &models.KnxDevice{Name: deviceConfig.Name}

	switch strings.ToLower(deviceConfig.Type) {
	case "sensor":
		device.Type = models.Sensor
	case "actor":
		device.Type = models.Actor
	default:
		return nil, fmt.Errorf("unknown KnxDevice type '%s'", deviceConfig.Type)
	}

	switch strings.ToLower(deviceConfig.ValueType) {
	case "temp":
		device.ValueType = models.Temperatur
	case "humidity":
		device.ValueType = models.Humidity
	case "wind":
		device.ValueType = models.Windspeed
	case "lux":
		device.ValueType = models.Brightness
	case "indicator":
		device.ValueType = models.Indicator
	case "shutter":
		device.ValueType = models.Shutter
		var windClass int
		switch strings.ToLower(deviceConfig.TypeConfig.WindClass) {
		case "low":
			windClass = models.WindClass{}.Low()
		case "medium":
			windClass = models.WindClass{}.Medium()
		case "high":
			windClass = models.WindClass{}.High()
		default:
			windClass = models.WindClass{}.Low()
			fmt.Printf("Warning: wind class %s not defined, falling back to 'low' for shutter %s", deviceConfig.TypeConfig.WindClass, deviceConfig.Name)
		}
		device.ShutterDevice = models.ShutterDevice{
			WindClass: windClass,
		}
	default:
		return nil, fmt.Errorf("unknown KnxDevice valuetype '%s'", deviceConfig.ValueType)
	}

	room := getRoomFromString(deviceConfig.Room)
	if room == "" {
		return nil, fmt.Errorf("unknown KnxDevice room '%s'", deviceConfig.Room)
	}
	device.Room = room

	return device, nil
}

func getRoomFromString(room string) string {

	switch strings.ToLower(room) {
	case "kitchen":
		return models.Kitchen
	case "terrace":
		return models.Terrace
	case "livingroom":
		return models.LivingRoom
	case "bathroomlarge":
		return models.BathroomLarge
	case "bathroomsmall":
		return models.BathroomSmall
	case "dining":
		return models.Dining
	case "officepat":
		return models.OfficePat
	case "officesteffi":
		return models.OfficeSteffi
	case "entry":
		return models.Entry
	case "bedroom":
		return models.Bedroom
	case "coridor":
		return models.Coridor
	case "reduit":
		return models.Reduit
	default:
		return ""
	}
}
