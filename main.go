package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"home_automation/internal/interfaces"
	"home_automation/internal/utils"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vapourismo/knx-go/knx/util"
)

func listenToKNX(knxClient interfaces.KnxClient, gauges utils.PromGauges) {
	go func() {
		// Receive messages from the gateway. The inbound channel is closed with the connection.
		for msg := range knxClient.Inbound() {
			interfaces.ProcessKNXMessage(msg, gauges)
		}
	}()
}

func fetchShellyData(gauges utils.PromGauges) {
	go func() {
		// Periodically fetch data for all shellies
		for range time.Tick(time.Second * 5) {
			for knxAddr, shellyDevice := range interfaces.KnxShellyMap {
				switchStatusResponse, err := shellyDevice.GetStatus()
				if err != nil {
					util.Logger.Printf("Error received from GetStatus call: %s. Skipping device %s", err, shellyDevice.Name)
					continue
				}
				switchStatus := switchStatusResponse.Switches[0]
				gauges.CurrentGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*switchStatus.Current)
				gauges.VoltageGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*switchStatus.Voltage)
				gauges.PowerConsumptionGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*switchStatus.APower)
				gauges.WifiSignalGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*switchStatusResponse.Wifi.RRSI)
				gauges.ShellyTempGauge.WithLabelValues(knxAddr, shellyDevice.Room, shellyDevice.Name, shellyDevice.Ip).Set(*switchStatus.Temperature.C)
			}
		}
	}()
}

func main() {
	var configFile string
	flag.StringVar(&configFile, "c", "config.yaml", "Specify the config file to be used. Default is config.yaml")
	flag.Parse()

	config := utils.LoadConfig(configFile)
	if config == nil {
		fmt.Println("Config file not loaded, exiting")
		os.Exit(1)
	}
	gauges := utils.InitPrometheus()
	interfaces.InitShelly(*config)
	knxClient := interfaces.InitKnx(*config)

	if knxClient == nil {
		fmt.Println("Failed initializing knxClient, exiting")
		os.Exit(1)
	}

	defer knxClient.Close()

	listenToKNX(*knxClient, gauges)
	fetchShellyData(gauges)

	http.Handle(config.PromExporter.Path, promhttp.Handler())
	http.ListenAndServe(fmt.Sprintf(":%d", config.PromExporter.Port), nil)
}
