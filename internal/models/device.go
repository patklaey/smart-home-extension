package models

const (
	// Value Types
	Temperatur = iota
	Humidity
	Windspeed
	Brightness
	Relais
	Shutter
	Light
	Indicator
	Shelly
	Meter

	// Types
	Sensor
	Actor

	// Rooms
	LivingRoom    = "LivingRoom"
	Kitchen       = "Kitchen"
	Dining        = "Dining"
	OfficeSteffi  = "OfficeSteffi"
	OfficePat     = "OfficePat"
	BathroomSmall = "BathroomSmall"
	BathroomLarge = "BathroomLarge"
	Bedroom       = "Bedroom"
	Reduit        = "Reduit"
	Coridor       = "Coridor"
	Entry         = "Entry"
	Terrace       = "Terrace"
)

type KnxDevice struct {
	Type          int
	Name          string
	Room          string
	ValueType     int
	KnxAddress    string
	ShutterDevice ShutterDevice
}

type ShutterDevice struct {
	WindClass int
}

type WindClass struct{}

func (WindClass) Low() int {
	return 0
}

func (WindClass) Medium() int {
	return 1
}

func (WindClass) High() int {
	return 2
}
