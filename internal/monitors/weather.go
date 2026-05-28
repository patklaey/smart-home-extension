package monitors

import (
	"context"
	"fmt"
	"home_automation/internal/interfaces"
	"home_automation/internal/logger"
	"home_automation/internal/models"
	"home_automation/internal/utils"
	"sync"
	"time"

	"github.com/vapourismo/knx-go/knx/dpt"
)

// TODO: Track shutter position as reported by KNX and store it here to be used as reference to retract when high windspeeds are detected

const (
	// IBrick Memo Names
	MemoAllAutoSunBlindsDown = "AllAutoSunBlindsDown"
	MemoWindWarning          = "SunBlindsWindWarning"

	windHysteresisFactor = 0.9
)

type WeatherMonitor struct {
	PromClient           interfaces.PromClientInterface
	windStatus           WindStatus
	KnxClient            interfaces.KnxClientInterface
	IBrickClient         interfaces.IBricksClientInterface
	MeteoClient          interfaces.MeteoClientInterface
	windResetGracePeriod int
	knxDevices           map[string]*models.KnxDevice
}

type WindStatus struct {
	mutex                        sync.RWMutex
	windShutterUpLowThreshold    float64
	windShutterUpMedThreshold    float64
	windShutterUpHighThreshold   float64
	windShutterUpLowCheckActive  bool
	windShutterUpMedCheckActive  bool
	windShutterUpHighCheckActive bool
}

func InitWeatherMonitor(config *utils.Config, knxDevices map[string]*models.KnxDevice, pClient interfaces.PromClientInterface, kClient interfaces.KnxClientInterface, iBricksClient interfaces.IBricksClientInterface, meteoClient interfaces.MeteoClientInterface) *WeatherMonitor {
	return &WeatherMonitor{
		PromClient:           pClient,
		KnxClient:            kClient,
		IBrickClient:         iBricksClient,
		MeteoClient:          meteoClient,
		windResetGracePeriod: config.Weather.Windspeed.WindResetGracePeriod,
		knxDevices:           knxDevices,
		windStatus: WindStatus{
			windShutterUpLowThreshold:    config.Weather.Windspeed.ShutterUpLowThreshold,
			windShutterUpMedThreshold:    config.Weather.Windspeed.ShutterUpMedThreshold,
			windShutterUpHighThreshold:   config.Weather.Windspeed.ShutterUpHighThreshold,
			windShutterUpLowCheckActive:  true,
			windShutterUpMedCheckActive:  true,
			windShutterUpHighCheckActive: true,
		},
	}
}

func (monitor *WeatherMonitor) CheckShutterUp(windspeed float64) {
	windDirection := monitor.MeteoClient.GetWindDirection()
	windDirectionFactor := monitor.MeteoClient.GetWindDirectionFactor()
	windspeed = windspeed * windDirectionFactor
	logger.Trace("Current wind direction is %d and associated wind factor is %.2f resulting wind windspeed %.2f", windDirection, windDirectionFactor, windspeed)
	monitor.windStatus.mutex.Lock()
	defer monitor.windStatus.mutex.Unlock()
	switch {
	case windspeed >= monitor.windStatus.windShutterUpHighThreshold:
		if monitor.windStatus.windShutterUpHighCheckActive {
			err := monitor.shutterUp(models.WindClassHigh)
			if err == nil {
				monitor.windStatus.windShutterUpHighCheckActive = false
				logger.Info("Shutters for high wind retracted")
				monitor.setIBricksWindWarningMemo(models.WindClassHigh)
			} else {
				logger.Warning("Some or all shutters could not be retracted (trigger high wind)")
			}
		} else {
			logger.Trace("High shutter check deactivated, shutters already retracted")
		}
	case windspeed >= monitor.windStatus.windShutterUpMedThreshold:
		if monitor.windStatus.windShutterUpMedCheckActive {
			err := monitor.shutterUp(models.WindClassMedium)
			if err == nil {
				monitor.windStatus.windShutterUpMedCheckActive = false
				logger.Info("Shutters for medium wind retracted")
				monitor.setIBricksWindWarningMemo(models.WindClassMedium)
			} else {
				logger.Warning("Some or all shutters could not be retracted (trigger medium wind)")
			}
		} else {
			logger.Trace("Medium shutter check deactivated, shutters already retracted")
		}
	case windspeed >= monitor.windStatus.windShutterUpLowThreshold:
		if monitor.windStatus.windShutterUpLowCheckActive {
			err := monitor.shutterUp(models.WindClassLow)
			if err == nil {
				monitor.windStatus.windShutterUpLowCheckActive = false
				logger.Info("Shutters for low wind retracted")
				monitor.setIBricksWindWarningMemo(models.WindClassLow)
			} else {
				logger.Warning("Some or all shutters could not be retracted (trigger low wind)")
			}
		} else {
			logger.Trace("Low shutter check deactivated, shutters already retracted")
		}
	}
}

func (monitor *WeatherMonitor) StartFetchingMaxWindspeed(ctx context.Context, frequency int) {
	go func() {
		ticker := time.NewTicker(time.Minute * time.Duration(frequency))
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Get max wind value for the last minutes
				query := fmt.Sprintf("max_over_time(knx_weather_windspeed_kmh[%dm])", monitor.windResetGracePeriod)
				values, err := monitor.PromClient.Query(query)
				if err != nil {
					logger.Error("Failed to query prometheus, retrying in %d minute(s)", frequency)
					continue
				}
				switch len(values) {
				case 0:
					logger.Warning("Not received any result for max_over_time(knx_weather_windspeed_kmh[%dm]), retrying in %d minute(s)", monitor.windResetGracePeriod, frequency)
				case 1:
					logger.Debug("Max windspeed in the last %d minutes: %.2f", monitor.windResetGracePeriod, values[0])
					monitor.checkReactivateShutterUp(values[0])
				default:
					logger.Warning("More than one result for max_over_time(knx_weather_windspeed_kmh[%dm]) received (expected just one) - using first one to continue: %v", monitor.windResetGracePeriod, values)
					monitor.checkReactivateShutterUp(values[0])
				}
			case <-ctx.Done():
				logger.Info("Stopping max windspeed fetching routine")
				return
			}
		}
	}()
}

func (monitor *WeatherMonitor) checkReactivateShutterUp(maxWindspeed float64) {
	windDirection := monitor.MeteoClient.GetWindDirection()
	maxWindspeed = maxWindspeed * monitor.MeteoClient.GetWindDirectionFactor()
	logger.Trace("Current wind direction is %d and associated wind factor is %.2f resulting max windspeed %.2f", windDirection, monitor.MeteoClient.GetWindDirectionFactor(), maxWindspeed)
	monitor.windStatus.mutex.Lock()
	defer monitor.windStatus.mutex.Unlock()
	switch {
	case maxWindspeed <= monitor.windStatus.windShutterUpLowThreshold*windHysteresisFactor:
		logger.Trace("Windspeed %.2f lower than 90%% of low retraction threshold %.2f, reactivating all checks again", maxWindspeed, monitor.windStatus.windShutterUpLowThreshold*windHysteresisFactor)
		if monitor.windStatus.windShutterUpLowCheckActive && monitor.windStatus.windShutterUpMedCheckActive && monitor.windStatus.windShutterUpHighCheckActive {
			logger.Trace("All shutter up checks active, nothing to reactivate")
			return
		} else {
			monitor.windStatus.windShutterUpLowCheckActive = true
			monitor.windStatus.windShutterUpMedCheckActive = true
			monitor.windStatus.windShutterUpHighCheckActive = true
			logger.Debug("All shutter up checks reactivated")
			monitor.setIBricksWindWarningMemo(models.WindClassNone)
		}
	case maxWindspeed <= monitor.windStatus.windShutterUpMedThreshold*windHysteresisFactor:
		logger.Trace("Windspeed %.2f lower than 90%% of medium retraction threshold %.2f, reactivating high and medium checks again", maxWindspeed, monitor.windStatus.windShutterUpMedThreshold*windHysteresisFactor)
		if monitor.windStatus.windShutterUpMedCheckActive && monitor.windStatus.windShutterUpHighCheckActive {
			logger.Trace("Medium and high shutter up checks active, nothing to reactivate")
			return
		} else {
			monitor.windStatus.windShutterUpMedCheckActive = true
			monitor.windStatus.windShutterUpHighCheckActive = true
			logger.Debug("High and medium shutter up checks reactivated")
			monitor.setIBricksWindWarningMemo(models.WindClassLow)
		}
	case maxWindspeed <= monitor.windStatus.windShutterUpHighThreshold*windHysteresisFactor:
		logger.Trace("Windspeed %.2f lower than 90%% of high retraction threshold %.2f, reactivating high checks again", maxWindspeed, monitor.windStatus.windShutterUpHighThreshold*windHysteresisFactor)
		if monitor.windStatus.windShutterUpHighCheckActive {
			logger.Trace("High shutter up checks active, nothing to reactivate")
			return
		} else {
			monitor.windStatus.windShutterUpHighCheckActive = true
			logger.Debug("High shutter up checks reactivated")
			monitor.setIBricksWindWarningMemo(models.WindClassMedium)
		}
	}
}

func (monitor *WeatherMonitor) setIBricksWindWarningMemo(windWarning models.WindClass) {
	if windWarning < models.WindClassNone || windWarning > models.WindClassVeryHigh {
		logger.Error("Wind warning must be between 0 and 5 (got %d). Not setting '%s' memo on iBricks", windWarning, MemoWindWarning)
		return
	}
	err := monitor.IBrickClient.SetMemo(MemoWindWarning, windWarning)
	if err != nil {
		logger.Warning("Shutter checks reactivated but failed to set memo '%s' to %d on iBricks", MemoWindWarning, windWarning)
	} else {
		logger.Debug("Memo '%s' on iBricks set successfully to '%d'", MemoWindWarning, windWarning)
	}
}

func (monitor *WeatherMonitor) shutterUp(windClass models.WindClass) error {
	var lastError error
	for knxAddress, knxDevice := range monitor.knxDevices {
		if knxDevice.Type == models.Actor && knxDevice.ValueType == models.Shutter && knxDevice.ShutterDevice.WindClass <= windClass {
			err := monitor.KnxClient.SendMessageToKnx(knxAddress, dpt.DPT_1001(false).Pack())
			if err != nil {
				logger.Error("Failed to send shutterUp command for shutter %s (%s): %s\n", knxDevice.Name, knxAddress, err)
				lastError = err
			}
		}
	}

	// Set memo in bricks that some shutters are retracted now
	err := monitor.IBrickClient.SetMemo(MemoAllAutoSunBlindsDown, 0)
	if err != nil {
		logger.Warning("Could not set memo '%s' to 0 on iBricks - automatic extension of shutters might be impacted", MemoAllAutoSunBlindsDown)
	} else {
		logger.Debug("Memo '%s' on iBricks set successfully to 0", MemoAllAutoSunBlindsDown)
	}

	return lastError
}
