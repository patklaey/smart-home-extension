package interfaces

import (
	"home_automation/internal/logger"
	"home_automation/internal/models"
	"home_automation/internal/utils"

	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/dpt"
)

var KnxShellyMap = map[string]*models.ShellyDevice{}

func InitShelly(config utils.Config) {

	for _, deviceConfig := range config.Shelly.ShellyDevices {
		device, err := deviceConfig.ToShellyDevice()
		if err != nil {
			logger.Warning("Failed creating shelly device %s from config: %s\n", deviceConfig.Ip, err)
			continue
		}
		KnxShellyMap[deviceConfig.KnxAddress] = device
	}
}

func ShellyHandleKnxMessage(knxAddr string, msg knx.GroupEvent) {
	shellyDevice := KnxShellyMap[knxAddr]
	logger.Debug("Handlig shelly message for %+v", msg)
	if shellyDevice.Type == models.Relais {
		var relaisStateToSet dpt.DPT_1001
		relaisStateToSet.Unpack(msg.Data)
		relaisState, err := shellyDevice.SetRelaisValue(bool(relaisStateToSet))
		if err != nil {
			logger.Error("Failed to set relais value on device %s (%s): %s\n", shellyDevice.Name, shellyDevice.Ip, err)
			return
		}
		err = SendMessageToKnx(shellyDevice.KnxReturnAddress, dpt.DPT_1001(relaisState == 1).Pack())
		if err != nil {
			logger.Error("Warning: failed to send relais value back on KNX, but relais state (%d) set on shelly device!\n", relaisState)
		}
	}

}
