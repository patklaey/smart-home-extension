package utils

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PromExporterGauges struct {
	WindspeedGauge        prometheus.Gauge
	LuxGauge              prometheus.Gauge
	TempGauge             *prometheus.GaugeVec
	HumidityGauge         *prometheus.GaugeVec
	RainIndicator         prometheus.Gauge
	PowerConsumptionGauge *prometheus.GaugeVec
	VoltageGauge          *prometheus.GaugeVec
	CurrentGauge          *prometheus.GaugeVec
	ShellyTempGauge       *prometheus.GaugeVec
	WifiSignalGauge       *prometheus.GaugeVec
}

func InitPromExporter() PromExporterGauges {
	gauges := PromExporterGauges{}
	// Set prometheus vars
	gauges.WindspeedGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "knx_weather_windspeed_kmh",
		Help: "The current windspeed in km/h",
	})
	gauges.LuxGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "knx_weather_brightness_lux",
		Help: "The current brightness in lux",
	})
	gauges.TempGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "knx",
			Name:      "room_temperatur_C",
			Help:      "The room temperatur in degrees celsius",
		},
		[]string{"knxAddress", "roomName", "sensorName"},
	)
	gauges.HumidityGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "knx",
			Name:      "room_humidity_percentage",
			Help:      "The room humidity in percentage",
		},
		[]string{"knxAddress", "roomName", "sensorName"},
	)
	gauges.RainIndicator = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "knx_weather_rain_indicator",
		Help: "The indicator for current rain value",
	})
	gauges.PowerConsumptionGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "shelly",
			Name:      "plug_power_consumption_w",
			Help:      "The power consumption of the plug in W",
		},
		[]string{"knxAddress", "roomName", "sensorName", "ipAddress"},
	)
	gauges.VoltageGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "shelly",
			Name:      "plug_voltage_v",
			Help:      "The voltage of the plug in V",
		},
		[]string{"knxAddress", "roomName", "sensorName", "ipAddress"},
	)
	gauges.CurrentGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "shelly",
			Name:      "plug_current_a",
			Help:      "The current of the plug in A",
		},
		[]string{"knxAddress", "roomName", "sensorName", "ipAddress"},
	)
	gauges.ShellyTempGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "shelly",
			Name:      "temperature_c",
			Help:      "The temperature of the shelly device in degrees C",
		},
		[]string{"knxAddress", "roomName", "sensorName", "ipAddress"},
	)
	gauges.WifiSignalGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "shelly",
			Name:      "wifi_signal",
			Help:      "The signal strength of the WIFI for the shelly device",
		},
		[]string{"knxAddress", "roomName", "sensorName", "ipAddress"},
	)

	return gauges
}
