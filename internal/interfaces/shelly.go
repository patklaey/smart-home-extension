package interfaces

import (
	"fmt"
	"home_automation/internal/models"
	"home_automation/internal/utils"

	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/dpt"
	"github.com/vapourismo/knx-go/knx/util"
)

var KnxShellyMap = map[string]*models.ShellyDevice{}

func InitShelly(config utils.Config) {

	for _, deviceConfig := range config.Shelly.ShellyDevices {
		device, err := deviceConfig.ToShellyDevice()
		if err != nil {
			fmt.Printf("Failed creating shelly device %s from config: %s\n", deviceConfig.Ip, err)
		}
		KnxShellyMap[deviceConfig.KnxAddress] = device
	}
}

func ShellyHandleKnxMessage(knxAddr string, msg knx.GroupEvent) {
	shellyDevice := KnxShellyMap[knxAddr]
	util.Logger.Printf("Handlig shelly message for %+v", msg)
	if shellyDevice.Type == models.Relais {
		var relaisStateToSet dpt.DPT_1001
		relaisStateToSet.Unpack(msg.Data)
		relaisState, err := shellyDevice.SetRelaisValue(bool(relaisStateToSet))
		if err != nil {
			fmt.Printf("Failed to set relais value on device %s (%s): %s\n", shellyDevice.Name, shellyDevice.Ip, err)
			return
		}
		err = SendMessageToKnx(shellyDevice.KnxReturnAddress, dpt.DPT_1001(relaisState == 1).Pack())
		if err != nil {
			fmt.Printf("Warning: failed to send relais value back on KNX, but relais state (%d) set on shelly device!\n", relaisState)
		}
	}

}
