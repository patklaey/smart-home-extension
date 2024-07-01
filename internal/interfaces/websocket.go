package interfaces

import (
	"encoding/json"
	"fmt"
	"home_automation/internal/clients"
	"home_automation/internal/logger"
	"home_automation/internal/utils"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

var localShellyClient *clients.ShellyClient

func StartWebsocketServer(config *utils.Config, shellyClient *clients.ShellyClient) {
	localShellyClient = shellyClient
	go func() {
		var upgrader = websocket.Upgrader{
			ReadBufferSize:  config.Websocket.Upgrader.ReadBufferSize,
			WriteBufferSize: config.Websocket.Upgrader.WriteBufferSize,
		}
		http.HandleFunc(config.Websocket.Path, func(w http.ResponseWriter, r *http.Request) {

			socket, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				logger.Error("Error upgrading to websocket protocol: %s - request host: %s", err.Error(), r.Host)
				return
			}
			listen(socket)
		})
		http.ListenAndServe(fmt.Sprintf(":%d", config.Websocket.Port), nil)
	}()
}

func listen(conn *websocket.Conn) {
	for {
		// read a message
		_, messageContent, err := conn.ReadMessage()
		if err != nil {
			logger.Error(err.Error())
			return
		}

		logger.Trace("Websocket message received: %s", string(messageContent))
		var jsonMap map[string]interface{}
		err = json.Unmarshal(messageContent, &jsonMap)
		if err != nil {
			logger.Error("Could not unmarshall message to map: %s", err)
			return
		}

		if source, found := jsonMap["src"]; found {
			if strings.HasPrefix(source.(string), "shelly") {
				err = localShellyClient.HandleWebSocketMessage(messageContent)
				if err != nil {
					logger.Warning("The following message received on the websocket could not successfully be handled by the shelly client: %s", string(messageContent))
				} else {
					logger.Trace("Websocket message successfully processed by shelly client")
				}
				continue
			}
		}

		// if we reach this point, no destination client for the message found
		logger.Warning("The following message received on the websocket was not understood, ignoring it. %s", string(messageContent))
	}
}
