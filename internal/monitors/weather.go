package monitors

import (
	"home_automation/internal/clients"
	"home_automation/internal/logger"
	"home_automation/internal/models"
	"home_automation/internal/utils"
	"time"

	"github.com/vapourismo/knx-go/knx/dpt"
)

type WeatherMonitor struct {
	PromClient *clients.PromClient
	WindStatus *WindStatus
	KnxClient  *clients.KnxClient
}

type WindStatus struct {
	windShutterUpLowThreshold    float64
	windShutterUpMedThreshold    float64
	windShutterUpHighThreshold   float64
	windShutterUpLowCheckActive  bool
	windShutterUpMedCheckActive  bool
	windShutterUpHighCheckActive bool
}

func InitWeatherMonitor(config *utils.Config, pClient *clients.PromClient, kClient *clients.KnxClient) WeatherMonitor {
	return WeatherMonitor{
		PromClient: pClient,
		KnxClient:  kClient,
		WindStatus: &WindStatus{
			windShutterUpLowThreshold:    config.Weather.Windspeed.ShutteUpLow,
			windShutterUpMedThreshold:    config.Weather.Windspeed.ShutteUpMed,
			windShutterUpHighThreshold:   config.Weather.Windspeed.ShutteUpHigh,
			windShutterUpLowCheckActive:  true,
			windShutterUpMedCheckActive:  true,
			windShutterUpHighCheckActive: true,
		},
	}
}

func (monitor *WeatherMonitor) CheckShutterUp(windspeed float64) {
	switch {
	case windspeed >= monitor.WindStatus.windShutterUpHighThreshold:
		if monitor.WindStatus.windShutterUpHighCheckActive {
			err := monitor.shutterUp(models.WindClass{}.High())
			if err == nil {
				monitor.WindStatus.windShutterUpHighCheckActive = false
				logger.Info("Shutters for high wind retracted")
			} else {
				logger.Warning("Some or all shutters could not be retracted (trigger high wind)")
			}
		}
	case windspeed >= monitor.WindStatus.windShutterUpMedThreshold:
		if monitor.WindStatus.windShutterUpMedCheckActive {
			err := monitor.shutterUp(models.WindClass{}.Medium())
			if err == nil {
				monitor.WindStatus.windShutterUpMedCheckActive = false
				logger.Info("Shutters for medium wind retracted")
			} else {
				logger.Warning("Some or all shutters could not be retracted (trigger medium wind)")
			}
		}
	case windspeed >= monitor.WindStatus.windShutterUpLowThreshold:
		if monitor.WindStatus.windShutterUpLowCheckActive {
			err := monitor.shutterUp(models.WindClass{}.Low())
			if err == nil {
				monitor.WindStatus.windShutterUpLowCheckActive = false
				logger.Info("Shutters for low wind retracted")
			} else {
				logger.Warning("Some or all shutters could not be retracted (trigger low wind)")
			}
		}
	}
}

func (monitor *WeatherMonitor) StartFetchingMaxWindspeed(frequency int) {
	go func() {
		for range time.Tick(time.Minute * time.Duration(frequency)) {
			// Get max wind value for the last 15 minutes
			values, err := monitor.PromClient.Query("max_over_time(knx_weather_windspeed_kmh[15m])")
			if err != nil {
				logger.Error("Failed to query prometheus, retrying in %d minute(s)", frequency)
				continue
			}
			switch len(values) {
			case 0:
				logger.Warning("Not received any result for max_over_time(knx_weather_windspeed_kmh[15m]), retrying in %d minute(s)", frequency)
			case 1:
				logger.Debug("Max windspeed in the last 15 minutes: %.2f", values[0])
				monitor.checkReactivateShutterUp(values[0])
			default:
				logger.Warning("More than one result for max_over_time(knx_weather_windspeed_kmh[15m]) received (expected just one) - using first one to continue: %v", values)
				monitor.checkReactivateShutterUp(values[0])
			}
		}
	}()
}

func (monitor *WeatherMonitor) checkReactivateShutterUp(maxWindpeed float64) {
	switch {
	case maxWindpeed <= monitor.WindStatus.windShutterUpLowThreshold*0.9:
		logger.Trace("Windspeed %.2f lower than 90%% of low retraction threshold %.2f, reactivating all checks again", maxWindpeed, monitor.WindStatus.windShutterUpLowThreshold*0.9)
		if monitor.WindStatus.windShutterUpLowCheckActive && monitor.WindStatus.windShutterUpMedCheckActive && monitor.WindStatus.windShutterUpHighCheckActive {
			logger.Trace("All shutter up checks active, nothing to reactivate")
			return
		} else {
			monitor.WindStatus.windShutterUpLowCheckActive = true
			monitor.WindStatus.windShutterUpMedCheckActive = true
			monitor.WindStatus.windShutterUpHighCheckActive = true
			logger.Debug("All shutter up checks reactivated")
		}
	case maxWindpeed <= monitor.WindStatus.windShutterUpMedThreshold*0.9:
		logger.Trace("Windspeed %.2f lower than 90%% of medium retraction threshold %.2f, reactivating high and medium checks again", maxWindpeed, monitor.WindStatus.windShutterUpMedThreshold*0.9)
		if monitor.WindStatus.windShutterUpMedCheckActive && monitor.WindStatus.windShutterUpHighCheckActive {
			logger.Trace("Medium and high shutter up checks active, nothing to reactivate")
			return
		} else {
			monitor.WindStatus.windShutterUpMedCheckActive = true
			monitor.WindStatus.windShutterUpHighCheckActive = true
			logger.Debug("High and medium shutter up checks reactivated")
		}
	case maxWindpeed <= monitor.WindStatus.windShutterUpHighThreshold*0.9:
		logger.Trace("Windspeed %.2f lower than 90%% of high retraction threshold %.2f, reactivating high checks again", maxWindpeed, monitor.WindStatus.windShutterUpHighThreshold*0.9)
		if monitor.WindStatus.windShutterUpMedCheckActive && monitor.WindStatus.windShutterUpHighCheckActive {
			logger.Trace("High shutter up checks active, nothing to reactivate")
			return
		} else {
			monitor.WindStatus.windShutterUpHighCheckActive = true
			logger.Debug("High shutter up checks reactivated")
		}
	}
}

func (monitor *WeatherMonitor) shutterUp(windClass int) error {
	var lastError error
	lastError = nil
	for knxAddress, knxDevice := range utils.KnxDevices {
		if knxDevice.Type == models.Actor && knxDevice.ValueType == models.Shutter && knxDevice.ShutterDevice.WindClass <= windClass {
			err := monitor.KnxClient.SendMessageToKnx(knxAddress, dpt.DPT_1001(false).Pack())
			if err != nil {
				logger.Error("Failed to send shutterUp command for shutter %s (%s): %s\n", knxDevice.Name, knxAddress, err)
				lastError = err
			}
		}
	}
	return lastError
}