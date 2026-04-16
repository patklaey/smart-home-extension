package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"home_automation/internal/clients"
	"home_automation/internal/interfaces"
	"home_automation/internal/logger"
	"home_automation/internal/monitors"
	"home_automation/internal/utils"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var healthStatus *utils.HealthStatus

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
	clients.InitClientVars(config)
	gauges := utils.InitPromExporter()
	healthStatus = utils.InitHealthStatus(gauges)
	iBricksClient := clients.InitIBricksClient(config)
	pClient := clients.InitPromClient()
	knxInterface := interfaces.InitAndConnectKnx(config)
	meteoClient := clients.InitMeteoClient(iBricksClient)
	shellyClient := clients.InitShelly(config, knxInterface.KnxClient, gauges)
	weatherMonitor := monitors.InitWeatherMonitor(config, pClient, knxInterface.KnxClient, iBricksClient, meteoClient)
	astronomyClient := clients.InitAstronomyClient(iBricksClient, config)
	interfaces.StartWebsocketServer(config, shellyClient)

	if knxInterface == nil {
		logger.Error("Failed initializing knxClient, exiting")
		os.Exit(1)
	}

	defer knxInterface.KnxClient.KnxTunnel.Close()

	knxInterface.ListenToKNX(gauges, weatherMonitor, shellyClient)
	knxInterface.MonitorKnxHealth(config.Knx.HealthCheckFrequencyMin, healthStatus)
	shellyClient.StartFetchShellyData(gauges, config.Shelly.ShellyPullFrequencySeconds)
	weatherMonitor.StartFetchingMaxWindspeed(config.Weather.Windspeed.CheckAverageFrequency)
	meteoClient.StratFetchingWindStatus()
	iBricksClient.StartSendingHeartbeat(config.IBricks.HeartbeatFrequency)
	astronomyClient.StartUpdatingSunAzimuth(config.Ipgeolocation.FetchFrequency)
	http.Handle(config.PromExporter.Path, promhttp.Handler())
	http.HandleFunc("/health", getHealthStatus)
	http.ListenAndServe(fmt.Sprintf(":%d", config.PromExporter.Port), nil)
}

func getHealthStatus(w http.ResponseWriter, r *http.Request) {
	healthStatusValue := healthStatus.GetHealthStatus()
	if healthStatusValue != 1 {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	io.WriteString(w, strconv.Itoa(healthStatus.GetHealthStatus()))
}
