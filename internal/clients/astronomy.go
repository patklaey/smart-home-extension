package clients

import (
	"context"
	"time"

	"home_automation/internal/logger"
	"home_automation/internal/utils"

	"github.com/carlmjohnson/requests"
)

// See https://ipgeolocation.io/astronomy-api.html#fields-reference for more info on fields
type Location struct {
	Location_string       string  `json:"location_string,omitempty"`
	Continent_code        string  `json:"continent_code,omitempty"`
	Continent_name        string  `json:"continent_name,omitempty"`
	Country_code2         string  `json:"country_code2,omitempty"`
	Country_code3         string  `json:"country_code3,omitempty"`
	Country_name          string  `json:"country_name,omitempty"`
	Country_name_official string  `json:"country_name_official,omitempty"`
	Is_eu                 bool    `json:"locais_eution_string,omitempty"`
	State_prov            string  `json:"state_prov,omitempty"`
	State_code            string  `json:"state_code,omitempty"`
	District              string  `json:"district,omitempty"`
	City                  string  `json:"city,omitempty"`
	Locality              string  `json:"locality,omitempty"`
	Zipcode               string  `json:"zipcode,omitempty"`
	Latitude              float64 `json:"latitude"`
	Longitude             float64 `json:"longitude"`
}

type Astronomy struct {
	Date                         string  `json:"date"`
	Current_time                 string  `json:"current_time"`
	Sunrise                      string  `json:"sunrise"`
	Sunset                       string  `json:"sunset"`
	Sun_status                   string  `json:"sun_status"`
	Solar_noon                   string  `json:"solar_noon"`
	Day_length                   string  `json:"day_length"`
	Sun_altitude                 float64 `json:"sun_altitude"`
	Sun_distance                 float64 `json:"sun_distance"`
	Sun_azimuth                  float64 `json:"sun_azimuth"`
	Moonrise                     string  `json:"moonrise"`
	Moonset                      string  `json:"moonset"`
	Moon_status                  string  `json:"moon_status"`
	Moon_altitude                float64 `json:"moon_altitude"`
	Moon_distance                float64 `json:"moon_distance"`
	Moon_azimuth                 float64 `json:"moon_azimuth"`
	Moon_parallactic_angle       float64 `json:"moon_parallactic_angle"`
	Moon_phase                   string  `json:"moon_phase"`
	Moon_illumination_percentage string  `json:"moon_illumination_percentage"`
	Moon_angle                   float64 `json:"moon_angle"`
}

type AstronomyResponse struct {
	Location  Location  `json:"location"`
	Astronomy Astronomy `json:"astronomy"`
}

type AstronomyClient struct {
	astronomyAPIKey string
	iBricksClient   *IBricksClient
}

const (
	MemoSunAzimuth = "SmartHomeExtensionSunAzimuth"
	Latitude       = ""
	Longitude      = ""
)

func InitAstronomyClient(iBricksClient *IBricksClient, config *utils.Config) *AstronomyClient {
	return &AstronomyClient{
		iBricksClient:   iBricksClient,
		astronomyAPIKey: config.Ipgeolocation.ApiKey,
	}
}

func (astronomyClient *AstronomyClient) StartUpdatingSunAzimuth(frequency int) {
	go func() {
		// Send initial heartbeat to let ibricks now we're here, then every frequency minute
		for range time.Tick(time.Second * time.Duration(frequency)) {
			astronomyInfo, err := astronomyClient.getAstronomyInfo()
			if err != nil {
				logger.Error("Failed to get astronomy info, retrying in %d minutes", frequency)
			} else {
				astronomyClient.iBricksClient.SetMemo(MemoSunAzimuth, astronomyInfo.Astronomy.Sun_azimuth)
			}
		}
	}()
}

func (astronomyClient *AstronomyClient) getAstronomyInfo() (*AstronomyResponse, error) {
	var response *AstronomyResponse
	requestUrl := "https://api.ipgeolocation.io/v2/astronomy"
	reqBuilder := requests.URL(requestUrl).
		Param("apiKey", astronomyClient.astronomyAPIKey).
		Param("lat", Latitude).
		Param("long", Longitude).
		Accept("application/json").
		ToJSON(&response)
	err := reqBuilder.Fetch(context.Background())

	if err != nil {
		logger.Error("Failed to get astronomy info: %s", err)
		return nil, err
	}
	return response, nil
}
