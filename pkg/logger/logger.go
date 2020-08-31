package logger

import (
	"fmt"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"log"
	"os"
	"strings"
)

var logLevel int
var showTrace bool

// Log level constants
const (
	ErrorLevel = iota + 1
	WarningLevel
	InfoLevel
	DebugLevel
)

// QuietMode turns off all log output
func QuietMode() {
	logLevel = 0
}

// SetLogLevel is a helper function for setting logLevel int
func SetLogLevel(level int) {
	logLevel = level
}

// SetShowTrace is a helper function for setting showTrace bool
func SetShowTrace(trace bool) {
	showTrace = trace
}

// GetLogLevelByName is a helper function for returning level associated with log
// level string
func GetLogLevelByName(logName string) (int, error) {
	var err error
	if val, hasKey := logLevelMap[strings.ToLower(logName)]; hasKey {
		return val, nil
	}
	err = fmt.Errorf("%s is not a valid log level name", logName)
	return 0, err
}

// GetLogLevel returns current log level
func GetLogLevel() int {
	return logLevel
}

// logLevelMap maps strings to log level int
// allows for --LogLevel Debug at command line
var logLevelMap = map[string]int{
	"error":   ErrorLevel,
	"warning": WarningLevel,
	"info":    InfoLevel,
	"debug":   DebugLevel,
}

// Log sends a log message at a given level
func Log(level int, format string, v ...interface{}) {
	var prefix string
	for k, v := range logLevelMap {
		if v == level {
			prefix = fmt.Sprintf("%s ", strings.ToUpper(k))
			break
		}
	}
	if prefix == "" {
		Error("Log entry received with invalid level: %s\n", fmt.Sprintf(format, v...))
		return
	}
	if logLevel >= level {
		log.SetPrefix(prefix)
		log.Printf(format, v...)
	}
}

// Error reports unexpected behavior, likely to result in termination
func Error(format string, v ...interface{}) {
	Log(ErrorLevel, format, v...)
}

// Warning reports unexpected behavior, not necessarily resulting in termination
func Warning(format string, v ...interface{}) {
	Log(WarningLevel, format, v...)
}

// Info provides general purpose statements useful to end user
func Info(format string, v ...interface{}) {
	Log(InfoLevel, format, v...)
}

// Debug contains extra information helpful to developers
func Debug(format string, v ...interface{}) {
	Log(DebugLevel, format, v...)
}

// Trace outputs detailed packet traversal
func Trace(format string, v ...interface{}) {
	if showTrace {
		log.SetPrefix("TRACE ")
		log.Printf(format, v...)
	}
}

type loglevelCfg struct {
	Level string `description:"Log level: Error, Warning, Info or Debug" barevalue:"yes" required:"yes"`
}

func (cfg loglevelCfg) Prepare() error {
	var err error
	val, err := GetLogLevelByName(cfg.Level)
	if err != nil {
		return err
	}
	SetLogLevel(val)
	return nil
}

type traceCfg struct{}

func (cfg traceCfg) Prepare() error {
	SetShowTrace(true)
	return nil
}

func init() {
	logLevel = InfoLevel
	showTrace = false
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime)

	cmdline.AddConfigType("log-level", "Set specific log level output", loglevelCfg{}, false, false, false, nil)
	cmdline.AddConfigType("trace", "Enables packet tracing output", traceCfg{}, false, false, false, nil)
}
