package logger

import (
	"fmt"
	"strings"
	"time"
)

const (
	// Log Levels
	LevelTrace = iota
	LevelDebug
	LevelInfo
	LevelWarning
	LevelError
)

type LogLevel = uint

var logLevel LogLevel
var levelToString = map[uint]string{
	LevelTrace:   "TRACE",
	LevelDebug:   "DEBUG",
	LevelInfo:    "INFO",
	LevelWarning: "WARN",
	LevelError:   "ERROR",
}

func Trace(format string, args ...interface{}) {
	log(LevelTrace, format, args...)
}

func Debug(format string, args ...interface{}) {
	log(LevelDebug, format, args...)
}

func Info(format string, args ...interface{}) {
	log(LevelInfo, format, args...)
}

func Warning(format string, args ...interface{}) {
	log(LevelWarning, format, args...)
}

func Error(format string, args ...interface{}) {
	log(LevelError, format, args...)
}

func log(messageLevel LogLevel, format string, args ...interface{}) {
	if messageLevel >= logLevel {
		fmt.Printf("%s\t%s\t%s\n", time.Now().Local().Format(time.RFC3339), levelToString[messageLevel], fmt.Sprintf(format, args...))
	}
}

func InitLogger(configLogLevel string) {
	switch strings.ToLower(configLogLevel) {
	case "trace":
		logLevel = LevelTrace
	case "debug":
		logLevel = LevelDebug
	case "info":
		logLevel = LevelInfo
	case "warning":
		logLevel = LevelWarning
	case "error":
		logLevel = LevelError
	default:
		fmt.Printf("Warning: Unkown loglevel '%s', fallback to trace", configLogLevel)
		logLevel = LevelTrace
	}
}
