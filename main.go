package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"home_automation/internal/clients"
	"home_automation/internal/interfaces"
	"home_automation/internal/logger"
	"home_automation/internal/monitors"
	"home_automation/internal/utils"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "c", "config.yaml", "Specify the config file to be used. Default is config.yaml")
	flag.Parse()

	config := utils.LoadConfig(configFile)
	if config == nil {
		fmt.Println("Config file not loaded, exiting")
		os.Exit(1)
	}

	logger.InitLogger(config.LogLevel)
	gauges := utils.InitPromExporter()
	iBricksClient := clients.InitIBricksClient(config)
	pClient := clients.InitPromClient()
	knxInterface := interfaces.InitAndConnectKnx(config)
	shellyClient := clients.InitShelly(config, knxInterface.KnxClient, gauges)
	weatherMonitor := monitors.InitWeatherMonitor(config, pClient, knxInterface.KnxClient, iBricksClient)
	interfaces.StartWebsocketServer(config, shellyClient)

	if knxInterface == nil {
		logger.Error("Failed initializing knxClient, exiting")
		os.Exit(1)
	}

	defer knxInterface.KnxClient.KnxTunnel.Close()

	knxInterface.ListenToKNX(gauges, &weatherMonitor, shellyClient)
	shellyClient.StartFetchShellyData(gauges, config.Shelly.ShellyPullFrequencySeconds)
	weatherMonitor.StartFetchingMaxWindspeed(5)
	iBricksClient.StartSendingHeartbeat(5)

	http.Handle(config.PromExporter.Path, promhttp.Handler())
	http.ListenAndServe(fmt.Sprintf(":%d", config.PromExporter.Port), nil)
}
