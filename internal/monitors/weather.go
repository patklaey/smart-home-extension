package monitors

import (
	"context"
	"errors"
	"fmt"
	"home_automation/internal/interfaces"
	"home_automation/internal/logger"
	"home_automation/internal/models"
	"home_automation/internal/utils"
	"slices"
	"sync"
	"time"

	"github.com/vapourismo/knx-go/knx/dpt"
)

// TODO: Track shutter position as reported by KNX and store it here to be used as reference to retract when high windspeeds are detected
var (
	windClassToShutterPositionMap = map[models.WindClass]map[models.WindClass]float64{}
)

const (
	// IBrick Memo Names
	MemoAllAutoSunBlindsDown = "AllAutoSunBlindsDown"
	MemoWindWarning          = "SunBlindsWindWarning"
	shutterNamePrefix        = "SmartHomeExtensionShutterPosition"

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
	mutex                          sync.RWMutex
	windShutterUpVeryLowThreshold  float64
	windShutterUpLowThreshold      float64
	windShutterUpMedThreshold      float64
	windShutterUpHighThreshold     float64
	windShutterUpVeryHighThreshold float64
	currentWindClass               models.WindClass
}

func InitWeatherMonitor(config *utils.Config, knxDevices map[string]*models.KnxDevice, pClient interfaces.PromClientInterface, kClient interfaces.KnxClientInterface, iBricksClient interfaces.IBricksClientInterface, meteoClient interfaces.MeteoClientInterface) *WeatherMonitor {
	// Create ShutterPositionMap for each wind class based on the config
	windClassToShutterPositionMap = map[models.WindClass]map[models.WindClass]float64{
		models.WindClassNone:     config.Weather.WindClassToShutterPositionMap.None.ToMap(),
		models.WindClassVeryLow:  config.Weather.WindClassToShutterPositionMap.VeryLow.ToMap(),
		models.WindClassLow:      config.Weather.WindClassToShutterPositionMap.Low.ToMap(),
		models.WindClassMedium:   config.Weather.WindClassToShutterPositionMap.Medium.ToMap(),
		models.WindClassHigh:     config.Weather.WindClassToShutterPositionMap.High.ToMap(),
		models.WindClassVeryHigh: config.Weather.WindClassToShutterPositionMap.VeryHigh.ToMap(),
	}
	weatherMonitor := &WeatherMonitor{
		PromClient:           pClient,
		KnxClient:            kClient,
		IBrickClient:         iBricksClient,
		MeteoClient:          meteoClient,
		windResetGracePeriod: config.Weather.Windspeed.WindResetGracePeriod,
		knxDevices:           knxDevices,
		windStatus: WindStatus{
			windShutterUpVeryLowThreshold:  config.Weather.Windspeed.ShutterUpVeryLowThreshold,
			windShutterUpLowThreshold:      config.Weather.Windspeed.ShutterUpLowThreshold,
			windShutterUpMedThreshold:      config.Weather.Windspeed.ShutterUpMedThreshold,
			windShutterUpHighThreshold:     config.Weather.Windspeed.ShutterUpHighThreshold,
			windShutterUpVeryHighThreshold: config.Weather.Windspeed.ShutterUpVeryHighThreshold,
			currentWindClass:               models.WindClassVeryHigh, // Start with highest wind class to be sure that shutters are retracted at the start if windspeed is high
		},
	}
	// Fetch max windspeed at the start to adjust the current wind class and shutter positions accordingly
	if value, err := weatherMonitor.fetchMaxWindspeed(); err != nil {
		logger.Error("Failed to fetch max windspeed at the start: %s", err)
	} else {
		weatherMonitor.checkWindClassReset(value)
	}
	return weatherMonitor
}

func (monitor *WeatherMonitor) CheckWindClassChange(windspeed float64) {
	windDirection := monitor.MeteoClient.GetWindDirection()
	windDirectionFactor := monitor.MeteoClient.GetWindDirectionFactor()
	windspeed = windspeed * windDirectionFactor
	logger.Trace("Current wind direction is %d and associated wind factor is %.2f resulting wind windspeed %.2f", windDirection, windDirectionFactor, windspeed)
	// Get the wind class based on the current windspeed and thresholds
	monitor.windStatus.mutex.Lock()
	defer monitor.windStatus.mutex.Unlock()
	windclass := monitor.getWindclassBySpeed(windspeed)
	logger.Debug("Windclass based on windspeed (%.2f km/h) is %s", windspeed, windclass)
	if getWindwarningByWindClass(windclass) > getWindwarningByWindClass(monitor.windStatus.currentWindClass) {
		logger.Trace("Windspeed %.2f km/h corresponds to wind class %s, which is higher than current wind class %s, setting corresponding shutter positions", windspeed, windclass, monitor.windStatus.currentWindClass)
		err := monitor.setWindClass(windclass)
		if err != nil {
			logger.Error("Failed to set wind class: %v trying to retract shutter for wind class %s", err, windclass)
			err := monitor.shutterUp(windclass)
			if err != nil {
				logger.Error("Failed to retract shutters for wind class %s after failed wind class setting: %v", windclass, err)
			}
			return
		}
		logger.Info("Windclass %s set on iBricks and shutters adjusted accordingly", windclass)
	} else {
		logger.Trace("Windspeed %.2f km/h corresponds to wind class %s, which is lower or equal to current wind class %s, no action needed", windspeed, windclass, monitor.windStatus.currentWindClass)
	}
}

func (monitor *WeatherMonitor) setWindClass(windclass models.WindClass) error {
	// Set the new wind class on iBricks memo and trigger shutter positions accordingly
	monitor.setIBricksWindWarningMemo(windclass)
	if err := monitor.setShutterPositionByWindClass(windclass); err != nil {
		return err
	}
	monitor.windStatus.currentWindClass = windclass
	logger.Info("Shutter positions and windclass set according to wind class %s", windclass)
	return nil
}

func (monitor *WeatherMonitor) setShutterPositionByWindClass(windclass models.WindClass) error {
	var errors []error
	for knxAddress, knxDevice := range monitor.knxDevices {
		if knxDevice.Type == models.Actor && knxDevice.ValueType == models.Shutter {
			// Get the shutter position based on the shutters wind class and the current wind class
			shutterPosition := windClassToShutterPositionMap[windclass][knxDevice.ShutterDevice.WindClass]
			logger.Trace("Shutter position for %s with wind class %s based on current wind class %s is %.2f", knxDevice.Name, knxDevice.ShutterDevice.WindClass, windclass, shutterPosition)
			// Check if the device has a correction factor configured and apply it if so
			if knxDevice.ShutterDevice.PositionCorrectionFactor > 0 && knxDevice.ShutterDevice.PositionCorrectionFactor < 1 {
				shutterPosition = shutterPosition * knxDevice.ShutterDevice.PositionCorrectionFactor
				logger.Trace("Applied correction factor %.2f to shutter %s resulting in new shutter position %.2f", knxDevice.ShutterDevice.PositionCorrectionFactor, knxDevice.Name, shutterPosition)
			} else if knxDevice.ShutterDevice.PositionCorrectionFactor > 1 {
				logger.Warning("Correction factor %.2f for shutter %s bigger than 1, ignoring it and using original position %.2f", knxDevice.ShutterDevice.PositionCorrectionFactor, knxDevice.Name, shutterPosition)
			}
			// Set the shutters position accordingly on iBricks
			err := monitor.setIBricksShutterPosition(knxDevice.Name, shutterPosition)
			if err != nil {
				logger.Error("Failed to send shutter position command for shutter %s (%s): %s\n", knxDevice.Name, knxAddress, err)
				errors = append(errors, err)
			}
		}
	}

	if err := monitor.IBrickClient.TriggerShutterPosition(); err != nil {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("At least one failure during attempt to update some shutter positions: %v", errors)
	}
	return nil
}

func (monitor *WeatherMonitor) getWindclassBySpeed(windspeed float64) models.WindClass {
	switch {
	case windspeed >= monitor.windStatus.windShutterUpVeryHighThreshold:
		return models.WindClassVeryHigh
	case windspeed >= monitor.windStatus.windShutterUpHighThreshold:
		return models.WindClassHigh
	case windspeed >= monitor.windStatus.windShutterUpMedThreshold:
		return models.WindClassMedium
	case windspeed >= monitor.windStatus.windShutterUpLowThreshold:
		return models.WindClassLow
	case windspeed >= monitor.windStatus.windShutterUpVeryLowThreshold:
		return models.WindClassVeryLow
	default:
		return models.WindClassNone
	}
}

func (monitor *WeatherMonitor) StartFetchingMaxWindspeed(ctx context.Context, frequency int) {
	go func() {
		ticker := time.NewTicker(time.Minute * time.Duration(frequency))
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				value, err := monitor.fetchMaxWindspeed()
				if err != nil {
					logger.Error("Failed to fetch max windspeed: %s", err)
					continue
				}
				monitor.checkWindClassReset(value)
			case <-ctx.Done():
				logger.Info("Stopping max windspeed fetching routine")
				return
			}
		}
	}()
}

func (monitor *WeatherMonitor) checkWindClassReset(maxWindspeed float64) {
	// Protect against unrealistic values
	if maxWindspeed < 0 {
		logger.Warning("Fetched max windspeed %.2f is smaller than 0, ignoring it", maxWindspeed)
		return
	}

	windDirection := monitor.MeteoClient.GetWindDirection()
	maxWindspeed = maxWindspeed * monitor.MeteoClient.GetWindDirectionFactor()
	logger.Trace("Current wind direction is %d and associated wind factor is %.2f resulting max windspeed %.2f", windDirection, monitor.MeteoClient.GetWindDirectionFactor(), maxWindspeed)
	monitor.windStatus.mutex.Lock()
	defer monitor.windStatus.mutex.Unlock()

	var affectedThresholdValue float64
	var affectedThresholdName string
	var windClassToSet models.WindClass

	switch {
	case maxWindspeed <= monitor.windStatus.windShutterUpVeryLowThreshold*windHysteresisFactor:
		affectedThresholdValue = monitor.windStatus.windShutterUpVeryLowThreshold
		affectedThresholdName = string(models.WindClassVeryLow)
		windClassToSet = models.WindClassNone
	case maxWindspeed <= monitor.windStatus.windShutterUpLowThreshold*windHysteresisFactor:
		affectedThresholdValue = monitor.windStatus.windShutterUpLowThreshold
		affectedThresholdName = string(models.WindClassLow)
		windClassToSet = models.WindClassVeryLow
	case maxWindspeed <= monitor.windStatus.windShutterUpMedThreshold*windHysteresisFactor:
		affectedThresholdValue = monitor.windStatus.windShutterUpMedThreshold
		affectedThresholdName = string(models.WindClassMedium)
		windClassToSet = models.WindClassLow
	case maxWindspeed <= monitor.windStatus.windShutterUpHighThreshold*windHysteresisFactor:
		affectedThresholdValue = monitor.windStatus.windShutterUpHighThreshold
		affectedThresholdName = string(models.WindClassHigh)
		windClassToSet = models.WindClassMedium
	case maxWindspeed <= monitor.windStatus.windShutterUpVeryHighThreshold*windHysteresisFactor:
		affectedThresholdValue = monitor.windStatus.windShutterUpVeryHighThreshold
		affectedThresholdName = string(models.WindClassVeryHigh)
		windClassToSet = models.WindClassHigh
	default:
		logger.Debug("Max windspeed %.2f is higher than very high threshold (%.2f), no action to trigger", maxWindspeed, monitor.windStatus.windShutterUpVeryHighThreshold*windHysteresisFactor)
		return
	}

	if getWindwarningByWindClass(monitor.windStatus.currentWindClass) > getWindwarningByWindClass(windClassToSet) {
		logger.Trace("Windspeed %.2f lower than 90%% of %s retraction threshold %.2f, setting windclass %s", maxWindspeed, affectedThresholdName, affectedThresholdValue*windHysteresisFactor, windClassToSet)
		if err := monitor.setWindClass(windClassToSet); err != nil {
			logger.Error("Failed to set (lower) wind class to %s: %s", windClassToSet, err)
		}
	} else if monitor.windStatus.currentWindClass == windClassToSet {
		logger.Trace("Current wind class and windclass to set are the same (%s), no action to trigger", windClassToSet)
	} else {
		logger.Warning("Windclass to set (%s), is higher than current wind class (%s) - this should not happen!", windClassToSet, monitor.windStatus.currentWindClass)
	}
}

func (monitor *WeatherMonitor) setIBricksWindWarningMemo(windclass models.WindClass) {
	validWindClasses := []models.WindClass{models.WindClassNone, models.WindClassVeryLow, models.WindClassLow, models.WindClassMedium, models.WindClassHigh, models.WindClassVeryHigh}
	if !slices.Contains(validWindClasses, windclass) {
		logger.Warning("Invalid wind class %s provided to setIBricksWindWarningMemo, valid values are: %v", windclass, validWindClasses)
		return
	}
	windWarning := getWindwarningByWindClass(windclass)
	err := monitor.IBrickClient.SetMemo(MemoWindWarning, windWarning)
	if err != nil {
		logger.Warning("Failed to set memo '%s' to %d on iBricks", MemoWindWarning, windWarning)
	} else {
		logger.Debug("Memo '%s' on iBricks set successfully to '%d'", MemoWindWarning, windWarning)
	}
}

func (monitor *WeatherMonitor) setIBricksShutterPosition(shutterName string, shutterPosition float64) error {
	memoName := fmt.Sprintf("%s-%s", shutterNamePrefix, shutterName)
	err := monitor.IBrickClient.SetMemo(memoName, shutterPosition)
	if err != nil {
		logger.Warning("Shutter checks reactivated but failed to set memo '%s' to %.2f on iBricks", memoName, shutterPosition)
	} else {
		logger.Debug("Memo '%s' on iBricks set successfully to '%.2f'", memoName, shutterPosition)
	}
	return err
}

func (monitor *WeatherMonitor) shutterUp(windClass models.WindClass) error {
	var lastError error
	for knxAddress, knxDevice := range monitor.knxDevices {
		if knxDevice.Type == models.Actor && knxDevice.ValueType == models.Shutter && getWindwarningByWindClass(knxDevice.ShutterDevice.WindClass) <= getWindwarningByWindClass(windClass) {
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

func (monitor *WeatherMonitor) fetchMaxWindspeed() (float64, error) {
	// Get max wind value for the last minutes
	query := fmt.Sprintf("max_over_time(knx_weather_windspeed_kmh[%dm])", monitor.windResetGracePeriod)
	values, err := monitor.PromClient.Query(query)
	if err != nil {
		logger.Error("Failed to query prometheus, retrying next cycle: %s", err)
		return 0, err
	}
	switch len(values) {
	case 0:
		logger.Warning("Not received any result for max_over_time(knx_weather_windspeed_kmh[%dm]), retrying next cycle", monitor.windResetGracePeriod)
		return 0, errors.New("no value received for max windspeed query")
	case 1:
		logger.Debug("Max windspeed in the last %d minutes: %.2f", monitor.windResetGracePeriod, values[0])
		return values[0], nil
	default:
		logger.Warning("More than one result for max_over_time(knx_weather_windspeed_kmh[%dm]) received (expected just one) - using first one to continue: %v", monitor.windResetGracePeriod, values)
		return values[0], nil
	}
}

func getWindwarningByWindClass(windclass models.WindClass) int {
	switch windclass {
	case models.WindClassNone:
		return 0
	case models.WindClassVeryLow:
		return 1
	case models.WindClassLow:
		return 2
	case models.WindClassMedium:
		return 3
	case models.WindClassHigh:
		return 4
	case models.WindClassVeryHigh:
		return 5
	default:
		logger.Warning("Invalid value '%s' for windclass, return max windwarning value (5)", windclass)
		return 5
	}
}
