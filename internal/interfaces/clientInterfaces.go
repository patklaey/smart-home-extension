package interfaces

import (
	"home_automation/internal/models"
	"home_automation/internal/utils"

	"github.com/vapourismo/knx-go/knx"
)

type IBricksClientInterface interface {
	SetMemo(memoName string, memoValue interface{}) error
	StartSendingHeartbeat(frequency int)
}

type MeteoClientInterface interface {
	StratFetchingWindStatus()
	GetWindDirectionFactor() float64
	GetWindDirection() int
}

type PromClientInterface interface {
	Query(metric string) ([]float64, error)
}

type KnxClientInterface interface {
	SendMessageToKnx(destination string, data []byte) error
}

type ShellyClientInterface interface {
	HandleKnxMessage(knxAddr string, msg knx.GroupEvent)
	HandleFullStatusMessageMessage(message *models.ShellyStatusUpdate) error
	StartFetchShellyData(gauges utils.PromExporterGauges, frequency int)
	HandleWebSocketMessage(messageContent []byte) error
	HandleStatusMessage(message *models.ShellyStatusUpdate) error
}
