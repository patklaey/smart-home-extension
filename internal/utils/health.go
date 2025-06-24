package utils

type HealthStatus struct {
	knxHealth int
}

var promGauges PromExporterGauges

func InitHealthStatus(gauges PromExporterGauges) *HealthStatus {
	promGauges = gauges
	return &HealthStatus{
		knxHealth: 0,
	}
}

func (healthStatus *HealthStatus) GetHealthStatus() int {
	return healthStatus.knxHealth
}

func (healthStatus *HealthStatus) SetKnxHealthStatus(health int) {
	healthStatus.knxHealth = health
	promGauges.KnxInterfaceHealth.Set(float64(health))
	promGauges.ApplicationHealth.Set(float64(healthStatus.GetHealthStatus()))
}
