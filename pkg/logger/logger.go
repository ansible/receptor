package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ghjm/cmdline"
	"github.com/spf13/viper"
)

var logLevel int

// logTimestampFormat matches the date/time layout produced by log.LstdFlags.
const logTimestampFormat = "2006/01/02 15:04:05"

// Log level constants.
const (
	ErrorLevel = iota + 1
	WarningLevel
	InfoLevel
	DebugLevel
	TraceLevel
)

// QuietMode turns off all log output.
func SetGlobalQuietMode() {
	logLevel = 0
}

// SetLogLevel is a helper function for setting logLevel int.
func SetGlobalLogLevel(level int) {
	logLevel = level
}

// GetLogLevelByName is a helper function for returning level associated with log
// level string.
func GetLogLevelByName(logName string) (int, error) {
	var err error
	if val, hasKey := logLevelMap[strings.ToLower(logName)]; hasKey {
		return val, nil
	}
	err = fmt.Errorf("%s is not a valid log level name", logName)

	return 0, err
}

// GetLogLevel returns current log level.
func GetLogLevel() int {
	return logLevel
}

// LogLevelToName takes an int and returns the corresponding log level name.
func LogLevelToName(logLevel int) (string, error) {
	var err error
	for k, v := range logLevelMap {
		if v == logLevel {
			return k, nil
		}
	}
	err = fmt.Errorf("%d is not a valid log level", logLevel)

	return "", err
}

// logLevelMap maps strings to log level int
// allows for --LogLevel Debug at command line.
var logLevelMap = map[string]int{
	"error":   ErrorLevel,
	"warning": WarningLevel,
	"info":    InfoLevel,
	"debug":   DebugLevel,
	"trace":   TraceLevel,
}

type MessageFunc func(level int, format string, v ...interface{})

var logger MessageFunc

// RegisterLogger registers a function for log delivery.
func RegisterLogger(msgFunc MessageFunc) {
	logger = msgFunc
}

// GetRegisteredLogger returns the current registered logger function.
func GetRegisteredLogger() MessageFunc {
	return logger
}

type ReceptorLogger struct {
	log.Logger
}

// NewReceptorLogger to instantiate a new logger object.
func NewReceptorLogger() *ReceptorLogger {
	return &ReceptorLogger{
		Logger: *log.New(os.Stdout, "", 0),
	}
}

// SetOutput sets the output destination for the logger.
func (rl *ReceptorLogger) SetOutput(w io.Writer) {
	rl.Logger.SetOutput(w)
}

// SetShowTrace enables trace-level logging.
func (rl *ReceptorLogger) SetShowTrace(trace bool) {
	SetGlobalLogLevel(TraceLevel)
}

// GetLogLevel returns the log level.
func (rl *ReceptorLogger) GetLogLevel() int {
	return logLevel
}

// Error reports unexpected behavior, likely to result in termination.
func (rl *ReceptorLogger) Error(format string, v ...interface{}) {
	rl.log(ErrorLevel, format, v...)
}

// SanError reports unexpected behavior, likely to result in termination.
func (rl *ReceptorLogger) SanitizedError(format string, v ...interface{}) {
	rl.sanitizedLog(ErrorLevel, format, v...)
}

// Warning reports unexpected behavior, not necessarily resulting in termination.
func (rl *ReceptorLogger) Warning(format string, v ...interface{}) {
	rl.log(WarningLevel, format, v...)
}

// SanitizedWarning reports unexpected behavior, not necessarily resulting in termination.
func (rl *ReceptorLogger) SanitizedWarning(format string, v ...interface{}) {
	rl.sanitizedLog(WarningLevel, format, v...)
}

// Info provides general purpose statements useful to end user.
func (rl *ReceptorLogger) Info(format string, v ...interface{}) {
	rl.log(InfoLevel, format, v...)
}

// SanitizedInfo provides general purpose statements useful to end user.
func (rl *ReceptorLogger) SanitizedInfo(format string, v ...interface{}) {
	rl.sanitizedLog(InfoLevel, format, v...)
}

// Debug contains extra information helpful to developers.
func (rl *ReceptorLogger) Debug(format string, v ...interface{}) {
	rl.log(DebugLevel, format, v...)
}

// Debug payload data.
func (rl *ReceptorLogger) DebugPayload(payloadDebug int, payload string, workUnitID string, connectionType string) {
	var payloadMessage string
	var workunitIDMessage string
	var connectionTypeMessage string
	switch payloadDebug {
	case 3:
		payloadMessage = fmt.Sprintf(" with a payload of: %s", payload)

		fallthrough
	case 2:
		if workUnitID != "" {
			workunitIDMessage = fmt.Sprintf(" with work unit %s", workUnitID)
		} else {
			workunitIDMessage = ", work unit not created yet"
		}

		fallthrough
	case 1:
		if connectionType != "" {
			connectionTypeMessage = fmt.Sprintf("Reading from %s", connectionType)
		}
	default:
	}
	rl.Debug("PACKET TRACING ENABLED: %s%s%s", connectionTypeMessage, workunitIDMessage, payloadMessage)
}

// SanitizedDebug contains extra information helpful to developers.
func (rl *ReceptorLogger) SanitizedDebug(format string, v ...interface{}) {
	rl.sanitizedLog(DebugLevel, format, v...)
}

// Trace outputs detailed packet traversal.
func (rl *ReceptorLogger) Trace(format string, v ...interface{}) {
	rl.log(TraceLevel, format, v...)
}

// SanitizedTrace outputs detailed packet traversal.
func (rl *ReceptorLogger) SanitizedTrace(format string, v ...interface{}) {
	rl.sanitizedLog(TraceLevel, format, v...)
}

// log prepends level and timestamp then prints a given log message.
func (rl *ReceptorLogger) log(level int, format string, v ...interface{}) {
	if logger != nil {
		logger(level, format, v...)

		return
	}
	logLevelName, err := LogLevelToName(level)
	if err != nil {
		rl.Error("Log entry received with invalid level: %s\n", fmt.Sprintf(format, v...))

		return
	}

	if logLevel >= level {
		now := time.Now().Format(logTimestampFormat)
		rl.Output(2, strings.ToUpper(logLevelName)+" "+now+" "+fmt.Sprintf(format, v...)) //nolint:errcheck
	}
}

// sanitizedLog prepends level and timestamp, strips embedded newlines, then prints a given log message.
func (rl *ReceptorLogger) sanitizedLog(level int, format string, v ...interface{}) {
	if logger != nil {
		logger(level, format, v...)

		return
	}
	logLevelName, err := LogLevelToName(level)
	if err != nil {
		message := fmt.Sprintf(format, v...)
		sanMessage := strings.ReplaceAll(message, "\n", "")
		rl.Error("Log entry received with invalid level: %s\n", sanMessage)

		return
	}

	if logLevel >= level {
		message := fmt.Sprintf(format, v...)
		sanMessage := strings.ReplaceAll(message, "\n", "")
		now := time.Now().Format(logTimestampFormat)
		rl.Output(2, strings.ToUpper(logLevelName)+" "+now+" "+sanMessage) //nolint:errcheck
	}
}

// GetLogLevelByName is a helper function for returning level associated with log
// level string.
func (rl *ReceptorLogger) GetLogLevelByName(logName string) (int, error) {
	var err error
	if val, hasKey := logLevelMap[strings.ToLower(logName)]; hasKey {
		return val, nil
	}
	err = fmt.Errorf("%s is not a valid log level name", logName)

	return 0, err
}

// LogLevelToName takes an int and returns the corresponding log level name.
func (rl *ReceptorLogger) LogLevelToName(logLevel int) (string, error) {
	var err error
	for k, v := range logLevelMap {
		if v == logLevel {
			return k, nil
		}
	}
	err = fmt.Errorf("%d is not a valid log level", logLevel)

	return "", err
}

type LoglevelCfg struct {
	Level string `description:"Log level: Error, Warning, Info, Debug or Trace" barevalue:"yes" default:"error"`
}

func (cfg LoglevelCfg) Init() error {
	var err error
	val, err := GetLogLevelByName(cfg.Level)
	if err != nil {
		return err
	}
	SetGlobalLogLevel(val)

	return nil
}

type TraceCfg struct{}

func (cfg TraceCfg) Prepare() error {
	SetGlobalLogLevel(TraceLevel)

	return nil
}

func init() {
	version := viper.GetInt("version")
	if version > 1 {
		return
	}
	logLevel = InfoLevel
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime)

	cmdline.RegisterConfigTypeForApp("receptor-logging",
		"log-level", "Specifies the verbosity level for command output", LoglevelCfg{}, cmdline.Singleton)
	cmdline.RegisterConfigTypeForApp("receptor-logging",
		"trace", "Enables packet tracing output", TraceCfg{}, cmdline.Singleton)
}
