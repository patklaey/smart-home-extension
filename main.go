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

	"github.com/gorilla/websocket"
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

	iBricksClient := clients.InitIBricksClient(config)
	pClient := clients.InitPromClient()
	knxInterface := interfaces.InitKnx(*config)
	logger.InitLogger(config.LogLevel)
	gauges := utils.InitPromExporter()
	shellyClient := clients.InitShelly(*config, knxInterface.KnxClient)
	weatherMonitor := monitors.InitWeatherMonitor(config, &pClient, knxInterface.KnxClient, iBricksClient)

	if knxInterface == nil {
		logger.Error("Failed initializing knxClient, exiting")
		os.Exit(1)
	}

	defer knxInterface.KnxClient.KnxTunnel.Close()

	knxInterface.ListenToKNX(gauges, &weatherMonitor, shellyClient)
	shellyClient.StartFetchShellyData(gauges)
	weatherMonitor.StartFetchingMaxWindspeed(1)
	iBricksClient.StartSendingHeartbeat(5)

	startWebsocketServer()

	http.Handle(config.PromExporter.Path, promhttp.Handler())
	http.ListenAndServe(fmt.Sprintf(":%d", config.PromExporter.Port), nil)
}

func startWebsocketServer() {
	go func() {
		var upgrader = websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}

		logger.Debug("in StartWebsocket")

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

			websocket, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				logger.Error(err.Error())
				return
			}
			logger.Debug("Websocket Connected!")
			listen(websocket)
		})
		http.ListenAndServe(":8088", nil)
	}()
}

func listen(conn *websocket.Conn) {
	for {
		// read a message
		messageType, messageContent, err := conn.ReadMessage()
		if err != nil {
			logger.Error(err.Error())
			return
		}

		// print out that message
		fmt.Printf("Message type: %d\n", messageType)
		fmt.Println(string(messageContent))
	}
}
