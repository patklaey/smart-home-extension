package utils

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PromGauges struct {
	WindspeedGauge   prometheus.Gauge
	LuxGauge         prometheus.Gauge
	TempGauge        *prometheus.GaugeVec
	RainIndicator    prometheus.Gauge
	PowerConsumption *prometheus.GaugeVec
	Voltage          *prometheus.GaugeVec
	Current          *prometheus.GaugeVec
	Test             prometheus.Gauge
}

func InitPrometheus() PromGauges {
	gauges := PromGauges{}
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
	gauges.RainIndicator = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "knx_weather_rain_indicator",
		Help: "The indicator for current rain value",
	})
	gauges.PowerConsumption = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "shelly",
			Name:      "plug_power_consumption_w",
			Help:      "The power consumption of the plug in W",
		},
		[]string{"knxAddress", "roomName", "sensorName", "ipAddress"},
	)
	gauges.Voltage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "shelly",
			Name:      "plug_voltage_v",
			Help:      "The voltage of the plug in V",
		},
		[]string{"knxAddress", "roomName", "sensorName", "ipAddress"},
	)
	gauges.Current = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "shelly",
			Name:      "plug_current_a",
			Help:      "The current of the plug in A",
		},
		[]string{"knxAddress", "roomName", "sensorName", "ipAddress"},
	)
	return gauges
}
