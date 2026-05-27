package clients

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"home_automation/internal/interfaces"
	mock_interfaces "home_automation/internal/mocks"
	"home_automation/internal/models"
	"home_automation/internal/utils"

	"github.com/golang/mock/gomock"
	goShelly "github.com/jcodybaker/go-shelly"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/cemi"
	"github.com/vapourismo/knx-go/knx/dpt"
)

// --- Helpers ---

func groupAddr(t *testing.T, addr string) cemi.GroupAddr {
	t.Helper()
	ga, err := cemi.NewGroupAddrString(addr)
	if err != nil {
		t.Fatalf("invalid group address %q: %v", addr, err)
	}
	return ga
}

func makeEvent(t *testing.T, dest string, data []byte) knx.GroupEvent {
	t.Helper()
	return knx.GroupEvent{
		Destination: groupAddr(t, dest),
		Data:        data,
	}
}

func strPtr(s string) *string   { return &s }
func f64Ptr(f float64) *float64 { return &f }

// newShellyClient constructs a ShellyClient with injected dependencies,
// bypassing InitShelly to avoid config/file dependencies in tests.
func newShellyClient(
	knxClient interfaces.KnxClientInterface,
	gauges utils.PromExporterGauges,
	knxDevices map[string]*models.KnxDevice,
	shellyMap map[string]*models.ShellyDevice,
) *ShellyClient {
	return &ShellyClient{
		knxClient:     knxClient,
		promGauges:    gauges,
		knxDevices:    knxDevices,
		shellyDevices: make(map[string]*models.ShellyDevice),
		shellyMap:     shellyMap,
	}
}

// newGaugeVec creates a real *prometheus.GaugeVec backed by an isolated registry.
func newGaugeVec(t *testing.T, name string, labels []string) *prometheus.GaugeVec {
	t.Helper()
	reg := prometheus.NewRegistry()
	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name}, labels)
	reg.MustRegister(g)
	return g
}

// gaugeVecValue reads the value of a GaugeVec for a specific set of label values.
func gaugeVecValue(t *testing.T, g *prometheus.GaugeVec, labels ...string) float64 {
	t.Helper()
	gauge, err := g.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("failed to get metric with labels %v: %v", labels, err)
	}
	ch := make(chan prometheus.Metric, 1)
	gauge.Collect(ch)
	var m dto.Metric
	if err := (<-ch).Write(&m); err != nil {
		t.Fatalf("failed to read gauge value: %v", err)
	}
	return m.GetGauge().GetValue()
}

// shellyServer starts a httptest.Server responding with the given status code and body,
// and returns a ShellyDevice pointing at it. Cleans up automatically via t.Cleanup.
func shellyServer(t *testing.T, statusCode int, body any) *models.ShellyDevice {
	t.Helper()
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal test response body: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write(bodyBytes)
	}))
	t.Cleanup(srv.Close)
	// Strip http:// since ShellyDevice.Ip is used to build the URL directly
	ip := srv.Listener.Addr().String()
	return &models.ShellyDevice{Name: "test-device", Ip: ip}
}

// --- HandleKnxMessage ---

func TestHandleKnxMessage_UnknownAddress_DoesNotCallKnx(t *testing.T) {
	ctrl := gomock.NewController(t)

	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
	knxClient.EXPECT().SendMessageToKnx(gomock.Any(), gomock.Any()).Times(0)

	sc := newShellyClient(knxClient, utils.PromExporterGauges{}, nil, map[string]*models.ShellyDevice{})
	sc.HandleKnxMessage("1/2/3", makeEvent(t, "1/2/3", []byte{0x01}))
}

func TestHandleKnxMessage_RelaisDevice_SendsReturnValueToKnx(t *testing.T) {
	ctrl := gomock.NewController(t)

	const knxAddr = "1/0/1"
	const returnAddr = "1/0/2"

	// Serve a valid relais response so SetRelaisValue succeeds
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models.ShellyRelaisActionResponse{IsOn: true})
	}))
	t.Cleanup(srv.Close)

	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
	knxClient.EXPECT().SendMessageToKnx(returnAddr, gomock.Any()).Times(1).Return(nil)

	sc := newShellyClient(knxClient, utils.PromExporterGauges{}, nil, map[string]*models.ShellyDevice{
		knxAddr: {
			Type:             models.Relais,
			Name:             "test relais",
			KnxReturnAddress: returnAddr,
			Ip:               srv.Listener.Addr().String(),
		},
	})
	sc.HandleKnxMessage(knxAddr, makeEvent(t, knxAddr, dpt.DPT_1001(true).Pack()))
}

func TestHandleKnxMessage_RelaisDevice_KnxSendFails_DoesNotPanic(t *testing.T) {
	ctrl := gomock.NewController(t)

	const knxAddr = "1/0/1"
	const returnAddr = "1/0/2"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models.ShellyRelaisActionResponse{IsOn: true})
	}))
	t.Cleanup(srv.Close)

	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
	knxClient.EXPECT().SendMessageToKnx(returnAddr, gomock.Any()).Times(1).Return(errors.New("knx send failed"))

	sc := newShellyClient(knxClient, utils.PromExporterGauges{}, nil, map[string]*models.ShellyDevice{
		knxAddr: {
			Type:             models.Relais,
			Name:             "test relais",
			KnxReturnAddress: returnAddr,
			Ip:               srv.Listener.Addr().String(),
		},
	})
	sc.HandleKnxMessage(knxAddr, makeEvent(t, knxAddr, dpt.DPT_1001(true).Pack()))
}

// --- GetStatus ---

func TestGetStatus_Success_ReturnsResponse(t *testing.T) {
	rrsi := -60.0
	expected := models.ShellyGetStatusResponse{
		Wifi: &goShelly.WifiStatus{RRSI: &rrsi},
	}
	device := shellyServer(t, http.StatusOK, expected)

	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, nil)
	resp, err := sc.GetStatus(device)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if *resp.Wifi.RRSI != rrsi {
		t.Errorf("expected RRSI=%.1f, got %.1f", rrsi, *resp.Wifi.RRSI)
	}
}

func TestGetStatus_ServerError_ReturnsError(t *testing.T) {
	device := shellyServer(t, http.StatusInternalServerError, nil)

	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, nil)
	_, err := sc.GetStatus(device)
	if err == nil {
		t.Error("expected error for server error response, got nil")
	}
}

func TestGetStatus_Unreachable_ReturnsError(t *testing.T) {
	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, nil)
	_, err := sc.GetStatus(&models.ShellyDevice{Name: "unreachable", Ip: "127.0.0.1:1"})
	if err == nil {
		t.Error("expected error for unreachable device, got nil")
	}
}

// --- SetRelaisValue ---

func TestSetRelaisValue_TurnOn_Success(t *testing.T) {
	device := shellyServer(t, http.StatusOK, models.ShellyRelaisActionResponse{IsOn: true})

	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, nil)
	state, err := sc.SetRelaisValue(device, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if state != 1 {
		t.Errorf("expected state=1, got %d", state)
	}
}

func TestSetRelaisValue_TurnOff_Success(t *testing.T) {
	device := shellyServer(t, http.StatusOK, models.ShellyRelaisActionResponse{IsOn: false})

	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, nil)
	state, err := sc.SetRelaisValue(device, false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if state != 0 {
		t.Errorf("expected state=0, got %d", state)
	}
}

func TestSetRelaisValue_ResponseMismatch_ReturnsError(t *testing.T) {
	// Requested on but device reports off — should return error
	device := shellyServer(t, http.StatusOK, models.ShellyRelaisActionResponse{IsOn: false})

	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, nil)
	_, err := sc.SetRelaisValue(device, true)
	if err == nil {
		t.Error("expected error when response state mismatches requested state, got nil")
	}
}

func TestSetRelaisValue_Unreachable_ReturnsError(t *testing.T) {
	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, nil)
	_, err := sc.SetRelaisValue(&models.ShellyDevice{Name: "unreachable", Ip: "127.0.0.1:1"}, true)
	if err == nil {
		t.Error("expected error for unreachable device, got nil")
	}
}

// --- HandleWebSocketMessage ---

func TestHandleWebSocketMessage_InvalidJson_ReturnsError(t *testing.T) {
	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, nil)
	if err := sc.HandleWebSocketMessage([]byte("not valid json")); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestHandleWebSocketMessage_UnknownMethod_ReturnsNil(t *testing.T) {
	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, nil)
	msg, _ := json.Marshal(&models.ShellyStatusUpdate{Method: "UnknownMethod"})
	if err := sc.HandleWebSocketMessage(msg); err != nil {
		t.Errorf("expected nil for unknown method, got %v", err)
	}
}

func TestHandleWebSocketMessage_NotifyFullStatus_DispatchesToHandleFullStatus(t *testing.T) {
	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, map[string]*models.ShellyDevice{})
	msg, _ := json.Marshal(&models.ShellyStatusUpdate{
		Method: models.ShellyNotifyFullStatus,
		Source: "unknown-source", // hits the default/ignore case in HandleFullStatusMessageMessage
	})
	if err := sc.HandleWebSocketMessage(msg); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestHandleWebSocketMessage_NotifyStatus_DispatchesToHandleStatus(t *testing.T) {
	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, nil)
	msg, _ := json.Marshal(&models.ShellyStatusUpdate{
		Method: models.ShellyNotifStatus,
		Source: "unknown-device", // not in shellyDevices, logs and returns nil
	})
	if err := sc.HandleWebSocketMessage(msg); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// --- HandleStatusMessage ---

func TestHandleStatusMessage_UnknownDevice_ReturnsNil(t *testing.T) {
	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, nil)
	if err := sc.HandleStatusMessage(&models.ShellyStatusUpdate{Source: "unknown"}); err != nil {
		t.Errorf("expected nil for unknown device, got %v", err)
	}
}

func TestHandleStatusMessage_KnownDevice_PM1_SetsGauges(t *testing.T) {
	const (
		knxAddr = "1/0/1"
		room    = "kitchen"
		name    = "pm1"
		ip      = "192.168.1.10"
		source  = "shellypmminig3-abc"
	)

	labels := []string{"knx_address", "room", "name", "ip"}
	voltageGauge := newGaugeVec(t, "voltage", labels)
	currentGauge := newGaugeVec(t, "current", labels)
	powerGauge := newGaugeVec(t, "power", labels)

	sc := newShellyClient(nil, utils.PromExporterGauges{
		VoltageGauge:          voltageGauge,
		CurrentGauge:          currentGauge,
		PowerConsumptionGauge: powerGauge,
	}, nil, nil)
	sc.shellyDevices[source] = &models.ShellyDevice{KnxAddress: knxAddr, Room: room, Name: name, Ip: ip}

	if err := sc.HandleStatusMessage(&models.ShellyStatusUpdate{
		Source: source,
		Parameters: &models.ShellyStatusUpdateParameters{
			PM1: &models.PM1{
				Voltage: f64Ptr(230.0),
				Current: f64Ptr(1.5),
				Apower:  f64Ptr(345.0),
			},
		},
	}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if got := gaugeVecValue(t, voltageGauge, knxAddr, room, name, ip); got != 230.0 {
		t.Errorf("expected voltage=230.0, got %.1f", got)
	}
	if got := gaugeVecValue(t, currentGauge, knxAddr, room, name, ip); got != 1.5 {
		t.Errorf("expected current=1.5, got %.1f", got)
	}
	if got := gaugeVecValue(t, powerGauge, knxAddr, room, name, ip); got != 345.0 {
		t.Errorf("expected power=345.0, got %.1f", got)
	}
}

// --- HandleFullStatusMessageMessage ---

func TestHandleFullStatusMessage_UnknownSource_ReturnsNil(t *testing.T) {
	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, map[string]*models.ShellyDevice{})
	if err := sc.HandleFullStatusMessageMessage(&models.ShellyStatusUpdate{Source: "totally-unknown"}); err != nil {
		t.Errorf("expected nil for unknown source, got %v", err)
	}
}

func TestHandleFullStatusMessage_PM1Mini_DeviceNotFound_ReturnsNil(t *testing.T) {
	sc := newShellyClient(nil, utils.PromExporterGauges{}, nil, map[string]*models.ShellyDevice{})
	err := sc.HandleFullStatusMessageMessage(&models.ShellyStatusUpdate{
		Source: "shellypmminig3-abc",
		Parameters: &models.ShellyStatusUpdateParameters{
			Wifi: &goShelly.WifiStatus{StaIP: strPtr("192.168.1.99")}, // not in shellyMap
		},
	})
	if err != nil {
		t.Errorf("expected nil when device not found, got %v", err)
	}
}

func TestHandleFullStatusMessage_HT_SendsTemperatureAndHumidityToKnx(t *testing.T) {
	ctrl := gomock.NewController(t)

	const tempAddr = "2/0/1"
	const humAddr = "2/0/2"
	const temp float64 = 21.5
	const humidity float64 = 55.0

	labels := []string{"knxAddress", "roomName", "sensorName"}
	tempGauge := newGaugeVec(t, "temperature", labels)
	humidityGauge := newGaugeVec(t, "humidity", labels)

	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
	knxClient.EXPECT().SendMessageToKnx(tempAddr, dpt.DPT_9001(temp).Pack()).Times(1).Return(nil)
	knxClient.EXPECT().SendMessageToKnx(humAddr, dpt.DPT_9001(humidity).Pack()).Times(1).Return(nil)

	sc := newShellyClient(knxClient, utils.PromExporterGauges{
		TempGauge:     tempGauge,
		HumidityGauge: humidityGauge,
	}, map[string]*models.KnxDevice{
		tempAddr: {Name: "Shelly H&T", Room: "bedroom", ValueType: models.Temperatur},
		humAddr:  {Name: "Shelly H&T", Room: "bedroom", ValueType: models.Humidity},
	}, map[string]*models.ShellyDevice{})

	if err := sc.HandleFullStatusMessageMessage(&models.ShellyStatusUpdate{
		Source: "shellyhtg3-abc",
		Parameters: &models.ShellyStatusUpdateParameters{
			Temperatures: models.ShellyTemperatureStatus{TC: temp},
			Humidities:   models.ShellyHumidityStatus{Humidity: humidity},
		},
	}); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestHandleFullStatusMessage_HT_KnxSendFails_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)

	const tempAddr = "2/0/1"
	const humAddr = "2/0/2"
	const temp float64 = 21.5
	const humidity float64 = 55.0
	const errorString = "knx send failed"

	labels := []string{"knxAddress", "roomName", "sensorName"}
	tempGauge := newGaugeVec(t, "temperature", labels)
	humidityGauge := newGaugeVec(t, "humidity", labels)

	knxClient := mock_interfaces.NewMockKnxClientInterface(ctrl)
	knxClient.EXPECT().SendMessageToKnx(tempAddr, dpt.DPT_9001(temp).Pack()).Times(1).Return(errors.New(errorString))
	knxClient.EXPECT().SendMessageToKnx(humAddr, dpt.DPT_9001(humidity).Pack()).Times(1).Return(nil)

	sc := newShellyClient(knxClient, utils.PromExporterGauges{
		TempGauge:     tempGauge,
		HumidityGauge: humidityGauge,
	}, map[string]*models.KnxDevice{
		tempAddr: {Name: "Shelly H&T", Room: "bedroom", ValueType: models.Temperatur},
		humAddr:  {Name: "Shelly H&T", Room: "bedroom", ValueType: models.Humidity},
	}, map[string]*models.ShellyDevice{})

	if err := sc.HandleFullStatusMessageMessage(&models.ShellyStatusUpdate{
		Source: "shellyhtg3-abc",
		Parameters: &models.ShellyStatusUpdateParameters{
			Temperatures: models.ShellyTemperatureStatus{TC: temp},
			Humidities:   models.ShellyHumidityStatus{Humidity: humidity},
		},
	}); err == nil {
		t.Error("expected error when KNX send fails, got nil")
	} else {
		assert.EqualError(t, err, errorString)
	}
}
