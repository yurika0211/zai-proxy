package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type LogLevel int32

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	currentLevel atomic.Int32
	levelNames   = map[LogLevel]string{
		DEBUG: "DEBUG",
		INFO:  "INFO",
		WARN:  "WARN",
		ERROR: "ERROR",
	}
	levelColors = map[LogLevel]string{
		DEBUG: "\033[36m",
		INFO:  "\033[32m",
		WARN:  "\033[33m",
		ERROR: "\033[31m",
	}
	resetColor = "\033[0m"
	output     io.Writer = os.Stdout
)

func init() {
	currentLevel.Store(int32(INFO))
}

func InitLogger(level string) {
	level = strings.TrimSpace(level)
	switch strings.ToLower(level) {
	case "debug":
		currentLevel.Store(int32(DEBUG))
	case "warn":
		currentLevel.Store(int32(WARN))
	case "error":
		currentLevel.Store(int32(ERROR))
	default:
		currentLevel.Store(int32(INFO))
	}
}

// SetOutput redirects log output; used in tests to suppress noise.
func SetOutput(w io.Writer) {
	output = w
}

func writeLog(level LogLevel, format string, v ...interface{}) {
	if level < LogLevel(currentLevel.Load()) {
		return
	}
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	msg := fmt.Sprintf(format, v...)
	// Use log.Printf to ensure thread-safe writes to output.
	logger := log.New(output, "", 0)
	logger.Printf("%s[%s]%s %s %s", levelColors[level], levelNames[level], resetColor, timestamp, msg)
}

func LogDebug(format string, v ...interface{}) {
	writeLog(DEBUG, format, v...)
}

func LogInfo(format string, v ...interface{}) {
	writeLog(INFO, format, v...)
}

func LogWarn(format string, v ...interface{}) {
	writeLog(WARN, format, v...)
}

func LogError(format string, v ...interface{}) {
	writeLog(ERROR, format, v...)
}