package monitors

import (
	"errors"
	"fmt"
	mock_interfaces "home_automation/internal/mocks"
	"home_automation/internal/models"
	"home_automation/internal/utils"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/vapourismo/knx-go/knx/dpt"
)

var shutterUpCommand = dpt.DPT_1001(false).Pack()

var windClassToShutterPositionMapTestData = &utils.WindClassToShutterPositionMap{
	None: &utils.ShutterPositionMap{
		Low:    100,
		Medium: 100,
		High:   100,
	},
	VeryLow: &utils.ShutterPositionMap{
		Low:    80,
		Medium: 100,
		High:   100,
	},
	Low: &utils.ShutterPositionMap{
		Low:    50,
		Medium: 80,
		High:   100,
	},
	Medium: &utils.ShutterPositionMap{
		Low:    0,
		Medium: 60,
		High:   100,
	},
	High: &utils.ShutterPositionMap{
		Low:    0,
		Medium: 0,
		High:   100,
	},
	VeryHigh: &utils.ShutterPositionMap{
		Low:    0,
		Medium: 0,
		High:   0,
	},
}

func TestInitWeatherMonitor(t *testing.T) {
	tests := []struct {
		name string
		want *WeatherMonitor
	}{
		{
			name: "Initialize WeatherMonitor with correct defaults",
			want: &WeatherMonitor{
				windResetGracePeriod: 60,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &utils.Config{
				Weather: &utils.WeatherConfig{
					Windspeed: &utils.WindspeedConfig{
						ShutterUpLowThreshold:  10.0,
						ShutterUpMedThreshold:  20.0,
						ShutterUpHighThreshold: 30.0,
						WindResetGracePeriod:   60,
					},
					WindClassToShutterPositionMap: windClassToShutterPositionMapTestData,
				},
			}

			ctrl := gomock.NewController(t)

			promClient := mock_interfaces.NewMockPromClientInterface(ctrl)
			knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
			iBricksClient := mock_interfaces.NewMockIBricksClientInterface(ctrl)
			meteoClient := mock_interfaces.NewMockMeteoClientInterface(ctrl)
			promClient.EXPECT().Query(gomock.Any()).Return(nil, errors.New("error")).Times(1)

			got := InitWeatherMonitor(config, map[string]*models.KnxDevice{}, promClient, knxClient, iBricksClient, meteoClient)

			if got.windResetGracePeriod != tt.want.windResetGracePeriod {
				t.Errorf("InitWeatherMonitor() windResetGracePeriod = %v, want %v", got.windResetGracePeriod, tt.want.windResetGracePeriod)
			}
			if got.PromClient == nil {
				t.Error("InitWeatherMonitor() PromClient is nil")
			}
			if got.KnxClient == nil {
				t.Error("InitWeatherMonitor() KnxClient is nil")
			}
			if got.IBrickClient == nil {
				t.Error("InitWeatherMonitor() IBrickClient is nil")
			}
			if got.MeteoClient == nil {
				t.Error("InitWeatherMonitor() MeteoClient is nil")
			}
			if got.windStatus.windShutterUpLowThreshold != 10.0 {
				t.Errorf("InitWeatherMonitor() windShutterUpLowThreshold = %v, want 10.0", got.windStatus.windShutterUpLowThreshold)
			}
			if got.windStatus.windShutterUpMedThreshold != 20.0 {
				t.Errorf("InitWeatherMonitor() windShutterUpMedThreshold = %v, want 20.0", got.windStatus.windShutterUpMedThreshold)
			}
			if got.windStatus.windShutterUpHighThreshold != 30.0 {
				t.Errorf("InitWeatherMonitor() windShutterUpHighThreshold = %v, want 30.0", got.windStatus.windShutterUpHighThreshold)
			}
			if got.windStatus.currentWindClass != models.WindClassVeryHigh {
				t.Errorf("InitWeatherMonitor() currentWindClass = %v, want %v", got.windStatus.currentWindClass, models.WindClassVeryHigh)
			}
		})
	}
}

func TestCheckWindClassChange_VeryLowWind(t *testing.T) {
	tests := []struct {
		name          string
		windspeed     float64
		wantWindClass models.WindClass
	}{
		{
			name:          "Windspeed below very low threshold",
			windspeed:     8.0,
			wantWindClass: models.WindClassNone,
		},
		{
			name:          "Windspeed triggers very low threshold",
			windspeed:     10.0,
			wantWindClass: models.WindClassVeryLow,
		},
		{
			name:          "Windspeed above very low threshold",
			windspeed:     15.0,
			wantWindClass: models.WindClassVeryLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			monitor := createTestMonitor(config, t, createTestShutters())
			iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
			monitor.windStatus.currentWindClass = models.WindClassNone // To test this, we need to start from wind class None
			if tt.windspeed >= 10.0 {
				//iBricksClient.EXPECT().SetMemo(MemoAllAutoSunBlindsDown, 0).Return(nil).Times(1)
				iBricksClient.EXPECT().SetMemo(MemoWindWarning, getWindwarningByWindClass(models.WindClassVeryLow)).Return(nil).Times(1)
				iBricksClient.EXPECT().TriggerShutterPosition().Return(nil).Times(1)
				memoName := fmt.Sprintf("%s-%s", shutterNamePrefix, "LowShutter")
				iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.VeryLow.Low)).Return(nil).Times(1)
				memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "MedShutter")
				iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.VeryLow.Medium)).Return(nil).Times(1)
				memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "HighShutter")
				iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.VeryLow.High)).Return(nil).Times(1)
			}

			monitor.CheckWindClassChange(tt.windspeed)
			assert.Equal(t, tt.wantWindClass, monitor.windStatus.currentWindClass)

		})
	}
}

func TestCheckWindClassChange_LowWind(t *testing.T) {
	tests := []struct {
		name          string
		windspeed     float64
		wantWindClass models.WindClass
	}{
		{
			name:          "Windspeed triggers low threshold",
			windspeed:     20.0,
			wantWindClass: models.WindClassLow,
		},
		{
			name:          "Windspeed above low threshold",
			windspeed:     25.0,
			wantWindClass: models.WindClassLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			monitor := createTestMonitor(config, t, createTestShutters())
			iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
			monitor.windStatus.currentWindClass = models.WindClassNone // To test this, we can start from wind class None
			iBricksClient.EXPECT().SetMemo(MemoWindWarning, getWindwarningByWindClass(tt.wantWindClass)).Return(nil).Times(1)
			iBricksClient.EXPECT().TriggerShutterPosition().Return(nil).Times(1)
			memoName := fmt.Sprintf("%s-%s", shutterNamePrefix, "LowShutter")
			iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Low.Low)).Return(nil).Times(1)
			memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "MedShutter")
			iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Low.Medium)).Return(nil).Times(1)
			memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "HighShutter")
			iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Low.High)).Return(nil).Times(1)

			monitor.CheckWindClassChange(tt.windspeed)
			assert.Equal(t, tt.wantWindClass, monitor.windStatus.currentWindClass)

		})
	}
}

func TestCheckWindClassChange_MediumWind(t *testing.T) {
	tests := []struct {
		name          string
		windspeed     float64
		wantWindClass models.WindClass
	}{
		{
			name:          "Windspeed triggers medium threshold",
			windspeed:     30.0,
			wantWindClass: models.WindClassMedium,
		},
		{
			name:          "Windspeed above medium threshold",
			windspeed:     35.0,
			wantWindClass: models.WindClassMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			monitor := createTestMonitor(config, t, createTestShutters())
			iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
			monitor.windStatus.currentWindClass = models.WindClassNone // To test this, we can start from wind class None
			iBricksClient.EXPECT().SetMemo(MemoWindWarning, getWindwarningByWindClass(tt.wantWindClass)).Return(nil).Times(1)
			iBricksClient.EXPECT().TriggerShutterPosition().Return(nil).Times(1)
			memoName := fmt.Sprintf("%s-%s", shutterNamePrefix, "LowShutter")
			iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Medium.Low)).Return(nil).Times(1)
			memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "MedShutter")
			iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Medium.Medium)).Return(nil).Times(1)
			memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "HighShutter")
			iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Medium.High)).Return(nil).Times(1)

			monitor.CheckWindClassChange(tt.windspeed)
			assert.Equal(t, tt.wantWindClass, monitor.windStatus.currentWindClass)

		})
	}
}

func TestCheckWindClassChange_HighWind(t *testing.T) {
	tests := []struct {
		name          string
		windspeed     float64
		wantWindClass models.WindClass
	}{
		{
			name:          "Windspeed triggers high threshold",
			windspeed:     40.0,
			wantWindClass: models.WindClassHigh,
		},
		{
			name:          "Windspeed above high threshold",
			windspeed:     45.0,
			wantWindClass: models.WindClassHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			monitor := createTestMonitor(config, t, createTestShutters())
			iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
			monitor.windStatus.currentWindClass = models.WindClassNone // To test this, we can start from wind class None
			iBricksClient.EXPECT().SetMemo(MemoWindWarning, getWindwarningByWindClass(tt.wantWindClass)).Return(nil).Times(1)
			iBricksClient.EXPECT().TriggerShutterPosition().Return(nil).Times(1)
			memoName := fmt.Sprintf("%s-%s", shutterNamePrefix, "LowShutter")
			iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.High.Low)).Return(nil).Times(1)
			memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "MedShutter")
			iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.High.Medium)).Return(nil).Times(1)
			memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "HighShutter")
			iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.High.High)).Return(nil).Times(1)

			monitor.CheckWindClassChange(tt.windspeed)
			assert.Equal(t, tt.wantWindClass, monitor.windStatus.currentWindClass)

		})
	}
}

func TestCheckWindClassChange_VeryHighWind(t *testing.T) {
	tests := []struct {
		name          string
		windspeed     float64
		wantWindClass models.WindClass
	}{
		{
			name:          "Windspeed triggers very high threshold",
			windspeed:     50.0,
			wantWindClass: models.WindClassVeryHigh,
		},
		{
			name:          "Windspeed above very high threshold",
			windspeed:     55.0,
			wantWindClass: models.WindClassVeryHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			monitor := createTestMonitor(config, t, createTestShutters())
			iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
			monitor.windStatus.currentWindClass = models.WindClassNone // To test this, we can start from wind class None
			iBricksClient.EXPECT().SetMemo(MemoWindWarning, getWindwarningByWindClass(tt.wantWindClass)).Return(nil).Times(1)
			iBricksClient.EXPECT().TriggerShutterPosition().Return(nil).Times(1)
			memoName := fmt.Sprintf("%s-%s", shutterNamePrefix, "LowShutter")
			iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.VeryHigh.Low)).Return(nil).Times(1)
			memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "MedShutter")
			iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.VeryHigh.Medium)).Return(nil).Times(1)
			memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "HighShutter")
			iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.VeryHigh.High)).Return(nil).Times(1)

			monitor.CheckWindClassChange(tt.windspeed)

			assert.Equal(t, tt.wantWindClass, monitor.windStatus.currentWindClass)

		})
	}
}

func TestCheckWindClassChange_SameWindclassDoesNotTriggerUpdate(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t, createTestShutters())
	monitor.windStatus.currentWindClass = models.WindClassMedium

	// However, a second call with the same wind class shouldn't trigger any changes
	monitor.CheckWindClassChange(34)
	assert.Equal(t, models.WindClassMedium, monitor.windStatus.currentWindClass)
}

func TestCheckWindClassChange_LowerWindclassDoesNotTriggerUpdate(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t, createTestShutters())

	monitor.windStatus.currentWindClass = models.WindClassMedium

	monitor.CheckWindClassChange(10)
	assert.Equal(t, models.WindClassMedium, monitor.windStatus.currentWindClass)
}

func TestCheckWindClassChange_FailedTiggerShutterPositionResultsInKNXShutterUpCommand(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t, createTestShutters())
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(MemoAllAutoSunBlindsDown, 0).Return(nil).Times(1)
	iBricksClient.EXPECT().SetMemo(MemoWindWarning, getWindwarningByWindClass(models.WindClassMedium)).Return(nil).Times(1)
	memoName := fmt.Sprintf("%s-%s", shutterNamePrefix, "LowShutter")
	iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Medium.Low)).Return(nil).Times(1)
	memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "MedShutter")
	iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Medium.Medium)).Return(nil).Times(1)
	memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "HighShutter")
	iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Medium.High)).Return(nil).Times(1)
	// Fail the trigger shutter position to test fallback to KNX command
	iBricksClient.EXPECT().TriggerShutterPosition().Return(errors.New("connection error")).Times(1)
	nKnxClient := monitor.KnxClient.(*mock_interfaces.MockKnxClientInterface)
	nKnxClient.EXPECT().SendMessageToKnx("1/2/3", shutterUpCommand).Return(nil).Times(1)
	nKnxClient.EXPECT().SendMessageToKnx("1/2/4", shutterUpCommand).Return(nil).Times(1)

	monitor.windStatus.currentWindClass = models.WindClassLow
	// First call to medium wind class tiggers above changes
	monitor.CheckWindClassChange(30)
	assert.Equal(t, models.WindClassLow, monitor.windStatus.currentWindClass)

}

func TestCheckWindClassReset_AllChecksReactivate(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t, createTestShutters())
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)

	// Set current wind class to low
	monitor.windStatus.currentWindClass = models.WindClassLow

	iBricksClient.EXPECT().SetMemo(MemoWindWarning, getWindwarningByWindClass(models.WindClassNone)).Return(nil).Times(1)
	memoName := fmt.Sprintf("%s-%s", shutterNamePrefix, "LowShutter")
	iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.None.Low)).Return(nil).Times(1)
	memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "MedShutter")
	iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.None.Medium)).Return(nil).Times(1)
	memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "HighShutter")
	iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.None.High)).Return(nil).Times(1)
	iBricksClient.EXPECT().TriggerShutterPosition().Return(nil).Times(1)

	// Wind drops below 90% of very low threshold
	monitor.checkWindClassReset(8.0)
	assert.Equal(t, models.WindClassNone, monitor.windStatus.currentWindClass)
}

func TestCheckWindClassReset_SameWindClassDoesNotChangeAnything(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t, createTestShutters())

	// Set current wind class to low
	monitor.windStatus.currentWindClass = models.WindClassLow

	// Wind stays within low windclass
	monitor.checkWindClassReset(20)
	assert.Equal(t, models.WindClassLow, monitor.windStatus.currentWindClass)
}

func TestCheckWindClassReset_HigherWindClassDoesNotChangeAnything(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t, createTestShutters())

	// Set current wind class to medium
	monitor.windStatus.currentWindClass = models.WindClassMedium

	// Wind higher than medium windclass
	monitor.checkWindClassReset(40)
	assert.Equal(t, models.WindClassMedium, monitor.windStatus.currentWindClass)
}

func TestCheckWindClassReset_FailedTriggerDoesNotTiggerAnyAction(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t, createTestShutters())
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(MemoWindWarning, getWindwarningByWindClass(models.WindClassLow)).Return(nil).Times(1)
	memoName := fmt.Sprintf("%s-%s", shutterNamePrefix, "LowShutter")
	iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Low.Low)).Return(nil).Times(1)
	memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "MedShutter")
	iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Low.Medium)).Return(nil).Times(1)
	memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "HighShutter")
	iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Low.High)).Return(nil).Times(1)
	iBricksClient.EXPECT().TriggerShutterPosition().Return(errors.New("an error occured")).Times(1)
	// Set current wind class to medium
	monitor.windStatus.currentWindClass = models.WindClassMedium

	// Wind drops below 90% of medium threshold
	monitor.checkWindClassReset(25)
	assert.Equal(t, models.WindClassMedium, monitor.windStatus.currentWindClass)
}

func TestSetIBricksWindWarningMemo_ValidValues(t *testing.T) {
	tests := []struct {
		name        string
		windWarning models.WindClass
		wantSuccess bool
	}{
		{
			name:        "Wind warning none",
			windWarning: models.WindClassNone,
			wantSuccess: true,
		},
		{
			name:        "Wind warning very low",
			windWarning: models.WindClassVeryLow,
			wantSuccess: true,
		},
		{
			name:        "Wind warning low",
			windWarning: models.WindClassLow,
			wantSuccess: true,
		},
		{
			name:        "Wind warning medium",
			windWarning: models.WindClassMedium,
			wantSuccess: true,
		},
		{
			name:        "Wind warning high",
			windWarning: models.WindClassHigh,
			wantSuccess: true,
		},
		{
			name:        "Wind warning very high",
			windWarning: models.WindClassVeryHigh,
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			monitor := createTestMonitor(config, t, nil)
			iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
			iBricksClient.EXPECT().SetMemo(MemoWindWarning, getWindwarningByWindClass(tt.windWarning)).Return(nil).Times(1)

			monitor.setIBricksWindWarningMemo(tt.windWarning)
		})
	}
}

func TestSetIBricksWindWarningMemo_InvalidValue(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t, nil)
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(gomock.Any(), gomock.Any()).Times(0)

	monitor.setIBricksWindWarningMemo("invalid wind class")
}

func TestSetIBricksWindWarningMemo_SetMemoError(t *testing.T) {
	ctrl := gomock.NewController(t)

	promClient := mock_interfaces.NewMockPromClientInterface(ctrl)
	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
	iBricksClient := mock_interfaces.NewMockIBricksClientInterface(ctrl)
	meteoClient := mock_interfaces.NewMockMeteoClientInterface(ctrl)

	iBricksClient.EXPECT().SetMemo(MemoWindWarning, getWindwarningByWindClass(models.WindClassLow)).Return(errors.New("connection error")).Times(1)

	monitor := &WeatherMonitor{
		PromClient:           promClient,
		KnxClient:            knxClient,
		IBrickClient:         iBricksClient,
		MeteoClient:          meteoClient,
		knxDevices:           map[string]*models.KnxDevice{},
		windResetGracePeriod: 60,
		windStatus: WindStatus{
			windShutterUpLowThreshold:  10.0,
			windShutterUpMedThreshold:  20.0,
			windShutterUpHighThreshold: 30.0,
		},
	}

	// Should handle error gracefully (already tested in real code)
	monitor.setIBricksWindWarningMemo(models.WindClassLow)
}

func TestShutterUp_NoShuttersMatch(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t, nil)
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(MemoAllAutoSunBlindsDown, 0).Return(nil).Times(1)

	err := monitor.shutterUp(models.WindClassLow)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestShutterUp_WithMatchingShutters(t *testing.T) {
	// Setup KnxDevices with matching shutters
	knxDevices := make(map[string]*models.KnxDevice)
	knxDevices["1/2/3"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "Shutter1",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClassLow,
		},
	}
	knxDevices["1/2/4"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "Shutter2",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClassLow,
		},
	}
	knxDevices["1/2/5"] = &models.KnxDevice{
		Type:      models.Sensor,
		Name:      "Temp",
		ValueType: models.Temperatur,
	}
	config := createTestConfig()
	monitor := createTestMonitor(config, t, knxDevices)
	nKnxClient := monitor.KnxClient.(*mock_interfaces.MockKnxClientInterface)
	nKnxClient.EXPECT().SendMessageToKnx("1/2/3", shutterUpCommand).Return(nil).Times(1)
	nKnxClient.EXPECT().SendMessageToKnx("1/2/4", shutterUpCommand).Return(nil).Times(1)
	nKnxClient.EXPECT().SendMessageToKnx("1/2/5", gomock.Any()).Return(nil).Times(0) // Should not be called for non-shutter
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(MemoAllAutoSunBlindsDown, 0).Return(nil).Times(1)

	err := monitor.shutterUp(models.WindClassLow)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestShutterUp_FilterByWindClass(t *testing.T) {
	// Setup KnxDevices with different wind classes
	knxDevices := make(map[string]*models.KnxDevice)
	knxDevices["1/2/3"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "LowShutter",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClassLow,
		},
	}
	knxDevices["1/2/4"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "MedShutter",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClassMedium,
		},
	}
	knxDevices["1/2/5"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "HighShutter",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClassHigh,
		},
	}

	config := createTestConfig()
	monitor := createTestMonitor(config, t, knxDevices)
	nKnxClient := monitor.KnxClient.(*mock_interfaces.MockKnxClientInterface)
	nKnxClient.EXPECT().SendMessageToKnx("1/2/3", shutterUpCommand).Return(nil).Times(3)
	nKnxClient.EXPECT().SendMessageToKnx("1/2/4", shutterUpCommand).Return(nil).Times(2)
	nKnxClient.EXPECT().SendMessageToKnx("1/2/5", shutterUpCommand).Return(nil).Times(1)
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(MemoAllAutoSunBlindsDown, 0).Return(nil).Times(3)

	// Trigger for high wind
	err := monitor.shutterUp(models.WindClassHigh)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Trigger for medium wind
	err = monitor.shutterUp(models.WindClassMedium)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Trigger for low wind
	err = monitor.shutterUp(models.WindClassLow)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

}

func TestShutterUp_KnxClientError(t *testing.T) {
	ctrl := gomock.NewController(t)

	promClient := mock_interfaces.NewMockPromClientInterface(ctrl)
	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
	iBricksClient := mock_interfaces.NewMockIBricksClientInterface(ctrl)
	meteoClient := mock_interfaces.NewMockMeteoClientInterface(ctrl)

	knxClient.EXPECT().SendMessageToKnx(gomock.Any(), gomock.Any()).Return(errors.New("connection error")).Times(1)
	iBricksClient.EXPECT().SetMemo(MemoAllAutoSunBlindsDown, 0).Return(nil).Times(1)

	// Setup a shutter
	knxDevices := make(map[string]*models.KnxDevice)
	knxDevices["1/2/3"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "Shutter1",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClassLow,
		},
	}

	monitor := &WeatherMonitor{
		PromClient:           promClient,
		KnxClient:            knxClient,
		IBrickClient:         iBricksClient,
		MeteoClient:          meteoClient,
		knxDevices:           knxDevices,
		windResetGracePeriod: 60,
		windStatus: WindStatus{
			windShutterUpLowThreshold:  10.0,
			windShutterUpMedThreshold:  20.0,
			windShutterUpHighThreshold: 30.0,
		},
	}

	err := monitor.shutterUp(models.WindClassLow)

	if err == nil {
		t.Error("Expected error from KnxClient, got nil")
	}
}

func TestWindStatus_InitialStateWithFailedInitialFetch(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t, nil)

	// Initialisation in createTestMonitor fails initial fetch of wind class due to Prometheus query error, so it should be set to VeryHigh as initialized
	assert.Equal(t, models.WindClassVeryHigh, monitor.windStatus.currentWindClass)
}

func TestWindStatus_InitialStateWithInitialFetch(t *testing.T) {
	ctrl := gomock.NewController(t)

	config := createTestConfig()
	devices := createTestShutters()

	promClient := mock_interfaces.NewMockPromClientInterface(ctrl)
	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
	iBricksClient := mock_interfaces.NewMockIBricksClientInterface(ctrl)
	meteoClient := mock_interfaces.NewMockMeteoClientInterface(ctrl)

	meteoClient.EXPECT().GetWindDirection().Return(0).AnyTimes()
	meteoClient.EXPECT().GetWindDirectionFactor().Return(1.0).AnyTimes()
	promClient.EXPECT().Query(gomock.Any()).Return([]float64{20.0}, nil).Times(1) // return low wind on initial fetch to not trigger shutter position update during initialization
	iBricksClient.EXPECT().SetMemo(MemoWindWarning, getWindwarningByWindClass(models.WindClassLow)).Return(nil).Times(1)
	memoName := fmt.Sprintf("%s-%s", shutterNamePrefix, "LowShutter")
	iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Low.Low)).Return(nil).Times(1)
	memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "MedShutter")
	iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Low.Medium)).Return(nil).Times(1)
	memoName = fmt.Sprintf("%s-%s", shutterNamePrefix, "HighShutter")
	iBricksClient.EXPECT().SetMemo(memoName, float32(config.Weather.WindClassToShutterPositionMap.Low.High)).Return(nil).Times(1)
	iBricksClient.EXPECT().TriggerShutterPosition().Return(nil).Times(1)

	monitor := InitWeatherMonitor(config, devices, promClient, knxClient, iBricksClient, meteoClient)
	assert.Equal(t, models.WindClassLow, monitor.windStatus.currentWindClass)
}

// func TestCheckWindClassChange_WithWindDirectionFactor(t *testing.T) {
// 	ctrl := gomock.NewController(t)

// 	promClient := mock_interfaces.NewMockPromClientInterface(ctrl)
// 	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
// 	iBricksClient := mock_interfaces.NewMockIBricksClientInterface(ctrl)
// 	meteoClient := mock_interfaces.NewMockMeteoClientInterface(ctrl)

// 	monitor := &WeatherMonitor{
// 		PromClient:           promClient,
// 		KnxClient:            knxClient,
// 		IBrickClient:         iBricksClient,
// 		MeteoClient:          meteoClient,
// 		knxDevices:           make(map[string]*models.KnxDevice),
// 		windResetGracePeriod: 60,
// 		windStatus: WindStatus{
// 			windShutterUpLowThreshold:    10.0,
// 			windShutterUpMedThreshold:    20.0,
// 			windShutterUpHighThreshold:   30.0,
// 			windShutterUpLowCheckActive:  true,
// 			windShutterUpMedCheckActive:  true,
// 			windShutterUpHighCheckActive: true,
// 		},
// 	}

// 	meteoClient.EXPECT().GetWindDirection().Return(0).Times(1)
// 	meteoClient.EXPECT().GetWindDirectionFactor().Return(0.5).Times(1)
// 	iBricksClient.EXPECT().SetMemo(MemoAllAutoSunBlindsDown, 0).Return(nil).Times(1)
// 	iBricksClient.EXPECT().SetMemo(MemoWindWarning, models.WindClassLow).Return(nil).Times(1)

// 	// Windspeed 20 with factor 0.5 = 10 (should trigger low threshold)
// 	monitor.CheckWindClassChange(20.0)

// 	if monitor.windStatus.windShutterUpLowCheckActive {
// 		t.Error("Expected low wind check to be deactivated")
// 	}
// }

// Helper functions

func createTestConfig() *utils.Config {
	return &utils.Config{
		Weather: &utils.WeatherConfig{
			Windspeed: &utils.WindspeedConfig{
				ShutterUpVeryLowThreshold:  10,
				ShutterUpLowThreshold:      20,
				ShutterUpMedThreshold:      30,
				ShutterUpHighThreshold:     40,
				ShutterUpVeryHighThreshold: 50,
				WindResetGracePeriod:       60,
			},
			WindClassToShutterPositionMap: windClassToShutterPositionMapTestData,
		},
	}
}

func createTestMonitor(config *utils.Config, t *testing.T, devices map[string]*models.KnxDevice) *WeatherMonitor {
	ctrl := gomock.NewController(t)

	if devices == nil {
		devices = make(map[string]*models.KnxDevice)
	}

	promClient := mock_interfaces.NewMockPromClientInterface(ctrl)
	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
	iBricksClient := mock_interfaces.NewMockIBricksClientInterface(ctrl)
	meteoClient := mock_interfaces.NewMockMeteoClientInterface(ctrl)

	meteoClient.EXPECT().GetWindDirection().Return(0).AnyTimes()
	meteoClient.EXPECT().GetWindDirectionFactor().Return(1.0).AnyTimes()
	promClient.EXPECT().Query(gomock.Any()).Return(nil, errors.New("error")).Times(1) // return error to not trigger initial shutter position update

	return InitWeatherMonitor(config, devices, promClient, knxClient, iBricksClient, meteoClient)
}

func createTestShutters() map[string]*models.KnxDevice {
	knxDevices := make(map[string]*models.KnxDevice)
	knxDevices["1/2/3"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "LowShutter",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClassLow,
		},
	}
	knxDevices["1/2/4"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "MedShutter",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClassMedium,
		},
	}
	knxDevices["1/2/5"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "HighShutter",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClassHigh,
		},
	}
	return knxDevices
}
