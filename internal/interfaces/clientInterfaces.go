package interfaces

import (
	"context"
	"home_automation/internal/models"
	"home_automation/internal/utils"

	"github.com/vapourismo/knx-go/knx"
)

type IBricksClientInterface interface {
	SetMemo(memoName string, memoValue interface{}) error
	TriggerShutterPosition() error
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
	StartFetchShellyData(ctx context.Context, gauges utils.PromExporterGauges, frequency int)
	HandleWebSocketMessage(messageContent []byte) error
	HandleStatusMessage(message *models.ShellyStatusUpdate) error
	GetStatus(actor *models.ShellyDevice) (*models.ShellyGetStatusResponse, error)
	SetRelaisValue(actor *models.ShellyDevice, value bool) (int, error)
}
