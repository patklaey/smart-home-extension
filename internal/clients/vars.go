package clients

import (
	"home_automation/internal/utils"
	"strconv"
)

const (
	// IBrick Memo Names
	MemoSunAzimuth         = "SmartHomeExtensionSunAzimuth"
	MemoSunAltitude        = "SmartHomeExtensionSunAltitude"
	MemoHeartbeatTimestamp = "SmartHomeExtensionHeartbeat"
	MemoWindDirection      = "SmartHomeExtensionWindDirection"
)

var (
	latitude  string
	longitude string
)

func InitClientVars(config *utils.Config) {
	latitude = strconv.FormatFloat(config.Weather.Location.Latitude, 'f', -1, 64)
	longitude = strconv.FormatFloat(config.Weather.Location.Longitude, 'f', -1, 64)
}
