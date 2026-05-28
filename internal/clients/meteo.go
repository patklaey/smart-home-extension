package clients

import (
	"context"
	"home_automation/internal/interfaces"
	"home_automation/internal/logger"
	"home_automation/internal/utils"
	"time"

	"github.com/carlmjohnson/requests"
)

const (
	METEO_BASE_URL = "https://api.open-meteo.com/v1/forecast"
)

var (
	currentWindDirection int
	windDirectionFactor  float64
)

type CurrentUnits struct {
	Time             string `json:"time,omitempty"`
	Interval         string `json:"interval,omitempty"`
	WindSpeed10m     string `json:"wind_speed_10m,omitempty"`
	WindDirection10m string `json:"wind_direction_10m,omitempty"`
	WindGusts10m     string `json:"wind_gusts_10m,omitempty"`
}

type Current struct {
	Time             string  `json:"time,omitempty"`
	Interval         int     `json:"interval,omitempty"`
	WindSpeed10m     float32 `json:"wind_speed_10m,omitempty"`
	WindDirection10m int     `json:"wind_direction_10m,omitempty"`
	WindGusts10m     float32 `json:"wind_gusts_10m,omitempty"`
}

type MeteoResponse struct {
	Latitude             float32      `json:"latitude,omitempty"`
	Longitude            float32      `json:"longitude,omitempty"`
	GenerationtimeMs     float32      `json:"generationtime_ms,omitempty"`
	UtcOffsetSeconds     int          `json:"utc_offset_seconds,omitempty"`
	Timezone             string       `json:"timezone,omitempty"`
	TimezoneAbbreviation string       `json:"timezone_abbreviation,omitempty"`
	Elevation            float32      `json:"elevation,omitempty"`
	CurrentUnits         CurrentUnits `json:"current_units,omitempty"`
	Current              Current      `json:"current,omitempty"`
}

type MeteoClient struct {
	iBricksClient interfaces.IBricksClientInterface
	config        *utils.Config
}

func InitMeteoClient(iBricksClient interfaces.IBricksClientInterface) *MeteoClient {
	client := &MeteoClient{
		iBricksClient: iBricksClient,
		config:        utils.GetConfig(),
	}
	client.fetchWindStatus()
	return client
}

func (meteoClient *MeteoClient) StratFetchingWindStatus() {
	go func() {
		// Send initial heartbeat to let ibricks now we're here, then every frequency minute
		for range time.Tick(time.Minute * time.Duration(meteoClient.config.Weather.Windspeed.CheckDirectionFrequency)) {
			meteoClient.fetchWindStatus()
		}
	}()
}

func (meteoClient *MeteoClient) fetchWindStatus() {
	response, err := meteoClient.getMeteoInfo()
	if err != nil {
		logger.Warning("Couldn't fetch wind status, keeping old wind direction value of %d", currentWindDirection)
		return
	}
	windDirection := response.Current.WindDirection10m
	if currentWindDirection != windDirection {
		meteoClient.iBricksClient.SetMemo(MemoWindDirection, windDirection)
		currentWindDirection = windDirection
		// TODO calculate wind direction factor and set it
		windDirectionFactor = 1.0
	}
}

func (meteoClient *MeteoClient) GetWindDirectionFactor() float64 {
	return windDirectionFactor
}

func (meteoClient *MeteoClient) GetWindDirection() int {
	return currentWindDirection
}

func (meteoClient *MeteoClient) getMeteoInfo() (*MeteoResponse, error) {
	logger.Trace("Getting current wind status from openmeteo")
	var response *MeteoResponse
	requestUrl := METEO_BASE_URL
	reqBuilder := requests.URL(requestUrl).
		Param("latitude", latitude).
		Param("longitude", longitude).
		Param("current", "wind_speed_10m,wind_direction_10m,wind_gusts_10m").
		Param("timezone", meteoClient.config.Weather.Location.Timezone).
		Param("past_days", "1").
		Accept("application/json").
		ToJSON(&response)
	err := reqBuilder.Fetch(context.Background())

	if err != nil {
		logger.Error("Failed to get meteo info: %s", err)
		return nil, err
	}
	logger.Debug("Current wind status %v", response)
	return response, nil
}
