package interfaces

import (
	"context"
)

// WeatherMonitorInterface defines the contract for the weather monitor,
// used by the KNX repository to decouple itself from the concrete implementation.
//
//go:generate mockgen -destination=../mocks/mock_weather_monitor.go -package=mocks home_automation/internal/interfaces WeatherMonitorInterface
type WeatherMonitorInterface interface {
	// CheckShutterUp evaluates the given windspeed and retracts shutters
	// for the appropriate wind class if thresholds are exceeded.
	CheckShutterUp(windspeed float64)

	// StartFetchingMaxWindspeed starts a background goroutine that periodically
	// queries Prometheus for the max windspeed and reactivates shutter checks
	// when wind drops below thresholds. The goroutine stops when ctx is cancelled.
	StartFetchingMaxWindspeed(ctx context.Context, frequency int)
}
