package repositories

import (
	"testing"

	mock_interfaces "home_automation/internal/mocks"
	"home_automation/internal/models"
	"home_automation/internal/utils"

	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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

func packWindspeed(t *testing.T, kmh float64) []byte {
	t.Helper()
	return dpt.DPT_9005(kmh).Pack()
}

func packIndicator(t *testing.T, value bool) []byte {
	t.Helper()
	return dpt.DPT_1002(value).Pack()
}

// newKnxInterface constructs a KnxInterface with the given device map,
// without needing a real KNX tunnel.
func newKnxInterface(devices map[string]*models.KnxDevice) *KnxInterface {
	return &KnxInterface{KnxDevices: devices}
}

// newRainGauge creates a real prometheus.Gauge backed by an isolated registry
// so tests don't share state via the default global registry.
func newRainGauge(t *testing.T) prometheus.Gauge {
	t.Helper()
	reg := prometheus.NewRegistry()
	g := prometheus.NewGauge(prometheus.GaugeOpts{Name: "rain_indicator"})
	reg.MustRegister(g)
	return g
}

// newWindspeedGauge creates a real prometheus.Gauge backed by an isolated registry
// so tests don't share state via the default global registry.
func newWindspeedGauge(t *testing.T) prometheus.Gauge {
	t.Helper()
	reg := prometheus.NewRegistry()
	g := prometheus.NewGauge(prometheus.GaugeOpts{Name: "windspeed_kmh"})
	reg.MustRegister(g)
	return g
}

// gaugeValue reads the current float64 value from a prometheus.Gauge.
func gaugeValue(t *testing.T, g prometheus.Gauge) float64 {
	t.Helper()
	ch := make(chan prometheus.Metric, 1)
	g.Collect(ch)
	var m dto.Metric
	if err := (<-ch).Write(&m); err != nil {
		t.Fatalf("failed to read gauge value: %v", err)
	}
	return m.GetGauge().GetValue()
}

// --- Windspeed ---

func TestProcessKNXMessage_Windspeed_CallsCheckShutterUp(t *testing.T) {
	ctrl := gomock.NewController(t)

	const addr = "1/1/1"
	const speed float64 = 42.0

	wm := mock_interfaces.NewMockWeatherMonitorInterface(ctrl)
	wm.EXPECT().CheckShutterUp(speed).Times(1)

	newKnxInterface(map[string]*models.KnxDevice{
		addr: {ValueType: models.Windspeed, Name: "wind", Room: "outside"},
	}).processKNXMessage(makeEvent(t, addr, packWindspeed(t, speed)), utils.PromExporterGauges{WindspeedGauge: newWindspeedGauge(t)}, wm, nil)
}

func TestProcessKNXMessage_Windspeed_UnpackError_DoesNotCallCheckShutterUp(t *testing.T) {
	ctrl := gomock.NewController(t)

	const addr = "1/1/1"

	wm := mock_interfaces.NewMockWeatherMonitorInterface(ctrl)
	wm.EXPECT().CheckShutterUp(gomock.Any()).Times(0)

	newKnxInterface(map[string]*models.KnxDevice{
		addr: {ValueType: models.Windspeed, Name: "wind", Room: "outside"},
	}).processKNXMessage(makeEvent(t, addr, []byte{0xFF, 0xFF, 0xFF, 0xFF}), utils.PromExporterGauges{}, wm, nil)
}

// --- Rain indicator ---

func TestProcessKNXMessage_Indicator_RainOn_SetsGaugeToOne(t *testing.T) {
	const addr = "3/0/1"
	rainGauge := newRainGauge(t)

	newKnxInterface(map[string]*models.KnxDevice{
		addr: {ValueType: models.Indicator, Name: "weatherstation", Room: "outside"},
	}).processKNXMessage(makeEvent(t, addr, packIndicator(t, true)), utils.PromExporterGauges{RainIndicator: rainGauge}, nil, nil)

	if got := gaugeValue(t, rainGauge); got != 1 {
		t.Errorf("expected RainIndicator=1, got %.0f", got)
	}
}

func TestProcessKNXMessage_Indicator_RainOff_SetsGaugeToZero(t *testing.T) {
	const addr = "3/0/1"
	rainGauge := newRainGauge(t)
	rainGauge.Set(1) // pre-set to confirm it gets reset

	newKnxInterface(map[string]*models.KnxDevice{
		addr: {ValueType: models.Indicator, Name: "weatherstation", Room: "outside"},
	}).processKNXMessage(makeEvent(t, addr, packIndicator(t, false)), utils.PromExporterGauges{RainIndicator: rainGauge}, nil, nil)

	if got := gaugeValue(t, rainGauge); got != 0 {
		t.Errorf("expected RainIndicator=0, got %.0f", got)
	}
}

func TestProcessKNXMessage_Indicator_NonWeatherstation_DoesNotChangeRainGauge(t *testing.T) {
	const addr = "3/0/2"
	rainGauge := newRainGauge(t)
	rainGauge.Set(1) // pre-set; should remain untouched

	newKnxInterface(map[string]*models.KnxDevice{
		addr: {ValueType: models.Indicator, Name: "some_other_sensor", Room: "garage"},
	}).processKNXMessage(makeEvent(t, addr, packIndicator(t, true)), utils.PromExporterGauges{RainIndicator: rainGauge}, nil, nil)

	if got := gaugeValue(t, rainGauge); got != 1 {
		t.Errorf("expected RainIndicator to remain 1, got %.0f", got)
	}
}

func TestProcessKNXMessage_Indicator_UnpackError_DoesNotChangeRainGauge(t *testing.T) {
	const addr = "3/0/1"
	rainGauge := newRainGauge(t)
	rainGauge.Set(1) // pre-set; should remain untouched on unpack failure

	newKnxInterface(map[string]*models.KnxDevice{
		addr: {ValueType: models.Indicator, Name: "weatherstation", Room: "outside"},
	}).processKNXMessage(makeEvent(t, addr, []byte{0xFF, 0xFF, 0xFF, 0xFF}), utils.PromExporterGauges{RainIndicator: rainGauge}, nil, nil)

	if got := gaugeValue(t, rainGauge); got != 1 {
		t.Errorf("expected RainIndicator to remain 1, got %.0f", got)
	}
}

// --- Shelly ---

func TestProcessKNXMessage_Shelly_Actor_CallsHandleKnxMessage(t *testing.T) {
	ctrl := gomock.NewController(t)

	const addr = "4/0/1"

	sc := mock_interfaces.NewMockShellyClientInterface(ctrl)
	sc.EXPECT().HandleKnxMessage(addr, gomock.Any()).Times(1)

	newKnxInterface(map[string]*models.KnxDevice{
		addr: {ValueType: models.Shelly, Type: models.Actor, Name: "shelly actor", Room: "hall"},
	}).processKNXMessage(makeEvent(t, addr, []byte{0x01}), utils.PromExporterGauges{}, nil, sc)
}

func TestProcessKNXMessage_Shelly_NonActor_DoesNotCallHandleKnxMessage(t *testing.T) {
	ctrl := gomock.NewController(t)

	const addr = "4/0/2"

	sc := mock_interfaces.NewMockShellyClientInterface(ctrl)
	sc.EXPECT().HandleKnxMessage(gomock.Any(), gomock.Any()).Times(0)

	newKnxInterface(map[string]*models.KnxDevice{
		addr: {ValueType: models.Shelly, Type: models.Sensor, Name: "shelly sensor", Room: "hall"},
	}).processKNXMessage(makeEvent(t, addr, []byte{0x01}), utils.PromExporterGauges{}, nil, sc)
}

// --- Unknown destination ---

func TestProcessKNXMessage_UnknownDestination_NoMocksAreCalled(t *testing.T) {
	ctrl := gomock.NewController(t)

	wm := mock_interfaces.NewMockWeatherMonitorInterface(ctrl)
	sc := mock_interfaces.NewMockShellyClientInterface(ctrl)
	wm.EXPECT().CheckShutterUp(gomock.Any()).Times(0)
	sc.EXPECT().HandleKnxMessage(gomock.Any(), gomock.Any()).Times(0)

	newKnxInterface(map[string]*models.KnxDevice{}).
		processKNXMessage(makeEvent(t, "1/2/3", []byte{0x00}), utils.PromExporterGauges{}, wm, sc)
}
