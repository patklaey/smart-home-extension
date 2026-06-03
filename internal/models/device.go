package models

type WindClass string

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

	// WindClasses
	WindClassNone     WindClass = "none"
	WindClassVeryLow  WindClass = "verylow"
	WindClassLow      WindClass = "low"
	WindClassMedium   WindClass = "medium"
	WindClassHigh     WindClass = "high"
	WindClassVeryHigh WindClass = "veryhigh"
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
	WindClass                WindClass
	PositionCorrectionFactor float64
}
