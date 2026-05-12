package monitors

import (
	"errors"
	mock_interfaces "home_automation/internal/mocks"
	"home_automation/internal/models"
	"home_automation/internal/utils"
	"testing"

	"github.com/golang/mock/gomock"
)

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
						ShutteUpLowThreshold:  10.0,
						ShutteUpMedThreshold:  20.0,
						ShutteUpHighThreshold: 30.0,
						WindResetGracePeriod:  60,
					},
				},
			}

			ctrl := gomock.NewController(t)

			promClient := mock_interfaces.NewMockPromClientInterface(ctrl)
			knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
			iBricksClient := mock_interfaces.NewMockIBricksClientInterface(ctrl)
			meteoClient := mock_interfaces.NewMockMeteoClientInterface(ctrl)

			got := InitWeatherMonitor(config, promClient, knxClient, iBricksClient, meteoClient)

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
			if got.WindStatus == nil {
				t.Fatal("InitWeatherMonitor() WindStatus is nil")
			}
			if got.WindStatus.windShutterUpLowThreshold != 10.0 {
				t.Errorf("InitWeatherMonitor() windShutterUpLowThreshold = %v, want 10.0", got.WindStatus.windShutterUpLowThreshold)
			}
			if got.WindStatus.windShutterUpMedThreshold != 20.0 {
				t.Errorf("InitWeatherMonitor() windShutterUpMedThreshold = %v, want 20.0", got.WindStatus.windShutterUpMedThreshold)
			}
			if got.WindStatus.windShutterUpHighThreshold != 30.0 {
				t.Errorf("InitWeatherMonitor() windShutterUpHighThreshold = %v, want 30.0", got.WindStatus.windShutterUpHighThreshold)
			}
			if !got.WindStatus.windShutterUpLowCheckActive {
				t.Error("InitWeatherMonitor() windShutterUpLowCheckActive should be true")
			}
			if !got.WindStatus.windShutterUpMedCheckActive {
				t.Error("InitWeatherMonitor() windShutterUpMedCheckActive should be true")
			}
			if !got.WindStatus.windShutterUpHighCheckActive {
				t.Error("InitWeatherMonitor() windShutterUpHighCheckActive should be true")
			}
		})
	}
}

func TestCheckShutterUp_LowWind(t *testing.T) {
	tests := []struct {
		name           string
		windspeed      float64
		wantLowActive  bool
		wantMedActive  bool
		wantHighActive bool
	}{
		{
			name:           "Windspeed below low threshold",
			windspeed:      8.0,
			wantLowActive:  true,
			wantMedActive:  true,
			wantHighActive: true,
		},
		{
			name:           "Windspeed triggers low threshold",
			windspeed:      10.0,
			wantLowActive:  false,
			wantMedActive:  true,
			wantHighActive: true,
		},
		{
			name:           "Windspeed above low threshold",
			windspeed:      15.0,
			wantLowActive:  false,
			wantMedActive:  true,
			wantHighActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			monitor := createTestMonitor(config, t)
			iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
			if tt.windspeed >= 10.0 {
				iBricksClient.EXPECT().SetMemo(MemoAllAusoSunBlindsDown, 0).Return(nil).Times(1)
				iBricksClient.EXPECT().SetMemo(MemoWindWarning, WindWarningLow).Return(nil).Times(1)
			}

			monitor.CheckShutterUp(tt.windspeed)

			if monitor.WindStatus.windShutterUpLowCheckActive != tt.wantLowActive {
				t.Errorf("CheckShutterUp() windShutterUpLowCheckActive = %v, want %v",
					monitor.WindStatus.windShutterUpLowCheckActive, tt.wantLowActive)
			}
			if monitor.WindStatus.windShutterUpMedCheckActive != tt.wantMedActive {
				t.Errorf("CheckShutterUp() windShutterUpMedCheckActive = %v, want %v",
					monitor.WindStatus.windShutterUpMedCheckActive, tt.wantMedActive)
			}
			if monitor.WindStatus.windShutterUpHighCheckActive != tt.wantHighActive {
				t.Errorf("CheckShutterUp() windShutterUpHighCheckActive = %v, want %v",
					monitor.WindStatus.windShutterUpHighCheckActive, tt.wantHighActive)
			}
		})
	}
}

func TestCheckShutterUp_MediumWind(t *testing.T) {
	tests := []struct {
		name           string
		windspeed      float64
		wantLowActive  bool
		wantMedActive  bool
		wantHighActive bool
	}{
		{
			name:           "Windspeed triggers medium threshold",
			windspeed:      20.0,
			wantLowActive:  true,
			wantMedActive:  false,
			wantHighActive: true,
		},
		{
			name:           "Windspeed above medium threshold",
			windspeed:      25.0,
			wantLowActive:  true,
			wantMedActive:  false,
			wantHighActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			monitor := createTestMonitor(config, t)
			iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
			iBricksClient.EXPECT().SetMemo(MemoAllAusoSunBlindsDown, 0).Return(nil).Times(1)
			iBricksClient.EXPECT().SetMemo(MemoWindWarning, WindWarningMedium).Return(nil).Times(1)

			monitor.CheckShutterUp(tt.windspeed)

			if monitor.WindStatus.windShutterUpLowCheckActive != tt.wantLowActive {
				t.Errorf("CheckShutterUp() windShutterUpLowCheckActive = %v, want %v",
					monitor.WindStatus.windShutterUpLowCheckActive, tt.wantLowActive)
			}
			if monitor.WindStatus.windShutterUpMedCheckActive != tt.wantMedActive {
				t.Errorf("CheckShutterUp() windShutterUpMedCheckActive = %v, want %v",
					monitor.WindStatus.windShutterUpMedCheckActive, tt.wantMedActive)
			}
			if monitor.WindStatus.windShutterUpHighCheckActive != tt.wantHighActive {
				t.Errorf("CheckShutterUp() windShutterUpHighCheckActive = %v, want %v",
					monitor.WindStatus.windShutterUpHighCheckActive, tt.wantHighActive)
			}
		})
	}
}

func TestCheckShutterUp_HighWind(t *testing.T) {
	tests := []struct {
		name           string
		windspeed      float64
		wantLowActive  bool
		wantMedActive  bool
		wantHighActive bool
	}{
		{
			name:           "Windspeed triggers high threshold",
			windspeed:      30.0,
			wantLowActive:  true,
			wantMedActive:  true,
			wantHighActive: false,
		},
		{
			name:           "Windspeed well above high threshold",
			windspeed:      50.0,
			wantLowActive:  true,
			wantMedActive:  true,
			wantHighActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			monitor := createTestMonitor(config, t)
			iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
			iBricksClient.EXPECT().SetMemo(MemoAllAusoSunBlindsDown, 0).Return(nil).Times(1)
			iBricksClient.EXPECT().SetMemo(MemoWindWarning, WindWarningHigh).Return(nil).Times(1)

			monitor.CheckShutterUp(tt.windspeed)

			if monitor.WindStatus.windShutterUpLowCheckActive != tt.wantLowActive {
				t.Errorf("CheckShutterUp() windShutterUpLowCheckActive = %v, want %v",
					monitor.WindStatus.windShutterUpLowCheckActive, tt.wantLowActive)
			}
			if monitor.WindStatus.windShutterUpMedCheckActive != tt.wantMedActive {
				t.Errorf("CheckShutterUp() windShutterUpMedCheckActive = %v, want %v",
					monitor.WindStatus.windShutterUpMedCheckActive, tt.wantMedActive)
			}
			if monitor.WindStatus.windShutterUpHighCheckActive != tt.wantHighActive {
				t.Errorf("CheckShutterUp() windShutterUpHighCheckActive = %v, want %v",
					monitor.WindStatus.windShutterUpHighCheckActive, tt.wantHighActive)
			}
		})
	}
}

func TestCheckShutterUp_AlreadyRetracted(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t)
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(MemoAllAusoSunBlindsDown, 0).Return(nil).Times(1)
	iBricksClient.EXPECT().SetMemo(MemoWindWarning, WindWarningMedium).Return(nil).Times(1)

	// First check triggers retraction
	monitor.CheckShutterUp(25.0)
	if monitor.WindStatus.windShutterUpMedCheckActive {
		t.Error("Expected medium check to be inactive after first check")
	}

	// Reset flag to false to simulate already retracted
	monitor.WindStatus.windShutterUpMedCheckActive = false

	// Second check shouldn't change anything since already retracted
	monitor.CheckShutterUp(25.0)

	if monitor.WindStatus.windShutterUpMedCheckActive {
		t.Error("Expected medium check to remain inactive")
	}
}

func TestCheckReactivateShutterUp_AllChecksReactivate(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t)
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(MemoWindWarning, WindWarningNone).Return(nil).Times(1)

	// Set all checks to false
	monitor.WindStatus.windShutterUpLowCheckActive = false
	monitor.WindStatus.windShutterUpMedCheckActive = false
	monitor.WindStatus.windShutterUpHighCheckActive = false

	// Wind drops below 90% of low threshold
	monitor.checkReactivateShutterUp(8.0)

	if !monitor.WindStatus.windShutterUpLowCheckActive {
		t.Error("Expected windShutterUpLowCheckActive to be true")
	}
	if !monitor.WindStatus.windShutterUpMedCheckActive {
		t.Error("Expected windShutterUpMedCheckActive to be true")
	}
	if !monitor.WindStatus.windShutterUpHighCheckActive {
		t.Error("Expected windShutterUpHighCheckActive to be true")
	}
}

func TestCheckReactivateShutterUp_MediumAndHighReactivate(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t)
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(MemoWindWarning, WindWarningLow).Return(nil).Times(1)

	// Set medium and high checks to false
	monitor.WindStatus.windShutterUpMedCheckActive = false
	monitor.WindStatus.windShutterUpHighCheckActive = false
	// Keep low check true

	// Wind drops below 90% of medium threshold but above 90% of low
	monitor.checkReactivateShutterUp(17.0)

	// Low should still be true (not changed)
	if !monitor.WindStatus.windShutterUpLowCheckActive {
		t.Error("Expected windShutterUpLowCheckActive to remain true")
	}
	// Medium and high should be reactivated
	if !monitor.WindStatus.windShutterUpMedCheckActive {
		t.Error("Expected windShutterUpMedCheckActive to be true")
	}
	if !monitor.WindStatus.windShutterUpHighCheckActive {
		t.Error("Expected windShutterUpHighCheckActive to be true")
	}
}

func TestCheckReactivateShutterUp_HighReactivate(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t)
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(MemoWindWarning, WindWarningMedium).Return(nil).Times(1)

	// Set high check to false only
	monitor.WindStatus.windShutterUpHighCheckActive = false
	// Keep medium and low checks true

	// Wind drops below 90% of high threshold but above 90% of medium
	monitor.checkReactivateShutterUp(25.0)

	// Low and medium should still be true (not changed)
	if !monitor.WindStatus.windShutterUpLowCheckActive {
		t.Error("Expected windShutterUpLowCheckActive to remain true")
	}
	if !monitor.WindStatus.windShutterUpMedCheckActive {
		t.Error("Expected windShutterUpMedCheckActive to remain true")
	}
	// Only high should be reactivated
	if !monitor.WindStatus.windShutterUpHighCheckActive {
		t.Error("Expected windShutterUpHighCheckActive to be true")
	}
}

func TestSetIBricksWindWarningMemo_ValidValues(t *testing.T) {
	tests := []struct {
		name        string
		windWarning int
		wantSuccess bool
	}{
		{
			name:        "Wind warning none",
			windWarning: WindWarningNone,
			wantSuccess: true,
		},
		{
			name:        "Wind warning very low",
			windWarning: WindWarningVeryLow,
			wantSuccess: true,
		},
		{
			name:        "Wind warning low",
			windWarning: WindWarningLow,
			wantSuccess: true,
		},
		{
			name:        "Wind warning medium",
			windWarning: WindWarningMedium,
			wantSuccess: true,
		},
		{
			name:        "Wind warning high",
			windWarning: WindWarningHigh,
			wantSuccess: true,
		},
		{
			name:        "Wind warning very high",
			windWarning: WindWarningVeryHigh,
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			monitor := createTestMonitor(config, t)
			iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
			iBricksClient.EXPECT().SetMemo(MemoWindWarning, tt.windWarning).Return(nil).Times(1)

			monitor.setIBricksWindWarningMemo(tt.windWarning)
		})
	}
}

func TestSetIBricksWindWarningMemo_InvalidValue(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t)
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(gomock.Any(), gomock.Any()).Times(0)

	monitor.setIBricksWindWarningMemo(999)
}

func TestSetIBricksWindWarningMemo_SetMemoError(t *testing.T) {
	ctrl := gomock.NewController(t)

	promClient := mock_interfaces.NewMockPromClientInterface(ctrl)
	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
	iBricksClient := mock_interfaces.NewMockIBricksClientInterface(ctrl)
	meteoClient := mock_interfaces.NewMockMeteoClientInterface(ctrl)

	iBricksClient.EXPECT().SetMemo(MemoWindWarning, WindWarningLow).Return(errors.New("connection error")).Times(1)

	monitor := &WeatherMonitor{
		PromClient:           promClient,
		KnxClient:            knxClient,
		IBrickClient:         iBricksClient,
		MeteoClient:          meteoClient,
		windResetGracePeriod: 60,
		WindStatus: &WindStatus{
			windShutterUpLowThreshold:    10.0,
			windShutterUpMedThreshold:    20.0,
			windShutterUpHighThreshold:   30.0,
			windShutterUpLowCheckActive:  true,
			windShutterUpMedCheckActive:  true,
			windShutterUpHighCheckActive: true,
		},
	}

	// Should handle error gracefully (already tested in real code)
	monitor.setIBricksWindWarningMemo(WindWarningLow)
}

func TestShutterUp_NoShuttersMatch(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t)
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(MemoAllAusoSunBlindsDown, 0).Return(nil).Times(1)

	// Clear KnxDevices
	utils.KnxDevices = make(map[string]*models.KnxDevice)

	err := monitor.shutterUp(models.WindClass{}.Low())

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestShutterUp_WithMatchingShutters(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t)
	nKnxClient := monitor.KnxClient.(*mock_interfaces.MockKnxClientInterface)
	nKnxClient.EXPECT().SendMessageToKnx("1/2/3", gomock.Any()).Return(nil).Times(1)
	nKnxClient.EXPECT().SendMessageToKnx("1/2/4", gomock.Any()).Return(nil).Times(1)
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(MemoAllAusoSunBlindsDown, 0).Return(nil).Times(1)

	// Setup KnxDevices with matching shutters
	utils.KnxDevices = make(map[string]*models.KnxDevice)
	utils.KnxDevices["1/2/3"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "Shutter1",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClass{}.Low(),
		},
	}
	utils.KnxDevices["1/2/4"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "Shutter2",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClass{}.Low(),
		},
	}
	utils.KnxDevices["1/2/5"] = &models.KnxDevice{
		Type:      models.Sensor,
		Name:      "Temp",
		ValueType: models.Temperatur,
	}

	err := monitor.shutterUp(models.WindClass{}.Low())

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestShutterUp_FilterByWindClass(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t)
	nKnxClient := monitor.KnxClient.(*mock_interfaces.MockKnxClientInterface)
	nKnxClient.EXPECT().SendMessageToKnx(gomock.Any(), gomock.Any()).Return(nil).Times(3)
	iBricksClient := monitor.IBrickClient.(*mock_interfaces.MockIBricksClientInterface)
	iBricksClient.EXPECT().SetMemo(MemoAllAusoSunBlindsDown, 0).Return(nil).Times(1)

	// Setup KnxDevices with different wind classes
	utils.KnxDevices = make(map[string]*models.KnxDevice)
	utils.KnxDevices["1/2/3"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "LowShutter",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClass{}.Low(),
		},
	}
	utils.KnxDevices["1/2/4"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "MedShutter",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClass{}.Medium(),
		},
	}
	utils.KnxDevices["1/2/5"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "HighShutter",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClass{}.High(),
		},
	}

	// Only trigger for high wind
	err := monitor.shutterUp(models.WindClass{}.High())

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
	iBricksClient.EXPECT().SetMemo(MemoAllAusoSunBlindsDown, 0).Return(nil).Times(1)

	monitor := &WeatherMonitor{
		PromClient:           promClient,
		KnxClient:            knxClient,
		IBrickClient:         iBricksClient,
		MeteoClient:          meteoClient,
		windResetGracePeriod: 60,
		WindStatus: &WindStatus{
			windShutterUpLowThreshold:    10.0,
			windShutterUpMedThreshold:    20.0,
			windShutterUpHighThreshold:   30.0,
			windShutterUpLowCheckActive:  true,
			windShutterUpMedCheckActive:  true,
			windShutterUpHighCheckActive: true,
		},
	}

	// Setup a shutter
	utils.KnxDevices = make(map[string]*models.KnxDevice)
	utils.KnxDevices["1/2/3"] = &models.KnxDevice{
		Type:      models.Actor,
		Name:      "Shutter1",
		ValueType: models.Shutter,
		ShutterDevice: models.ShutterDevice{
			WindClass: models.WindClass{}.Low(),
		},
	}

	err := monitor.shutterUp(models.WindClass{}.Low())

	if err == nil {
		t.Error("Expected error from KnxClient, got nil")
	}
}

func TestWindStatus_InitialState(t *testing.T) {
	config := createTestConfig()
	monitor := createTestMonitor(config, t)

	if !monitor.WindStatus.windShutterUpLowCheckActive {
		t.Error("Expected windShutterUpLowCheckActive to be true initially")
	}
	if !monitor.WindStatus.windShutterUpMedCheckActive {
		t.Error("Expected windShutterUpMedCheckActive to be true initially")
	}
	if !monitor.WindStatus.windShutterUpHighCheckActive {
		t.Error("Expected windShutterUpHighCheckActive to be true initially")
	}
}

func TestCheckShutterUp_WithWindDirectionFactor(t *testing.T) {
	ctrl := gomock.NewController(t)

	promClient := mock_interfaces.NewMockPromClientInterface(ctrl)
	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
	iBricksClient := mock_interfaces.NewMockIBricksClientInterface(ctrl)
	meteoClient := mock_interfaces.NewMockMeteoClientInterface(ctrl)

	monitor := &WeatherMonitor{
		PromClient:           promClient,
		KnxClient:            knxClient,
		IBrickClient:         iBricksClient,
		MeteoClient:          meteoClient,
		windResetGracePeriod: 60,
		WindStatus: &WindStatus{
			windShutterUpLowThreshold:    10.0,
			windShutterUpMedThreshold:    20.0,
			windShutterUpHighThreshold:   30.0,
			windShutterUpLowCheckActive:  true,
			windShutterUpMedCheckActive:  true,
			windShutterUpHighCheckActive: true,
		},
	}

	meteoClient.EXPECT().GetWindDirection().Return(0).Times(1)
	meteoClient.EXPECT().GetWindDirectionFactor().Return(0.5).Times(1)
	iBricksClient.EXPECT().SetMemo(MemoAllAusoSunBlindsDown, 0).Return(nil).Times(1)
	iBricksClient.EXPECT().SetMemo(MemoWindWarning, WindWarningLow).Return(nil).Times(1)

	utils.KnxDevices = make(map[string]*models.KnxDevice)

	// Windspeed 20 with factor 0.5 = 10 (should trigger low threshold)
	monitor.CheckShutterUp(20.0)

	if monitor.WindStatus.windShutterUpLowCheckActive {
		t.Error("Expected low wind check to be deactivated")
	}
}

// Helper functions

func createTestConfig() *utils.Config {
	return &utils.Config{
		Weather: &utils.WeatherConfig{
			Windspeed: &utils.WindspeedConfig{
				ShutteUpLowThreshold:  10.0,
				ShutteUpMedThreshold:  20.0,
				ShutteUpHighThreshold: 30.0,
				WindResetGracePeriod:  60,
			},
		},
	}
}

func createTestMonitor(config *utils.Config, t *testing.T) *WeatherMonitor {
	ctrl := gomock.NewController(t)

	promClient := mock_interfaces.NewMockPromClientInterface(ctrl)
	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
	iBricksClient := mock_interfaces.NewMockIBricksClientInterface(ctrl)
	meteoClient := mock_interfaces.NewMockMeteoClientInterface(ctrl)

	meteoClient.EXPECT().GetWindDirection().Return(0).AnyTimes()
	meteoClient.EXPECT().GetWindDirectionFactor().Return(1.0).AnyTimes()

	return InitWeatherMonitor(config, promClient, knxClient, iBricksClient, meteoClient)
}
