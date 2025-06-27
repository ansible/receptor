package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/ghjm/cmdline"
	"github.com/spf13/viper"
)

var (
	logLevel  int
	showTrace bool
)

// Log level constants.
const (
	ErrorLevel = iota + 1
	WarningLevel
	InfoLevel
	DebugLevel
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

// This doesn't seem to be used anywhere.
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
	Prefix string
	m      sync.Mutex
	Suffix map[string]string
}

// NewReceptorLogger to instantiate a new logger object.
func NewReceptorLogger(prefix string) *ReceptorLogger {
	return &ReceptorLogger{
		Logger: *log.New(os.Stdout, prefix, log.LstdFlags),
		Prefix: prefix,
	}
}

// NewReceptorLoggerWithSuffix to instantiate a new logger object with a new Suffix.
func NewReceptorLoggerWithSuffix(prefix string, suffix map[string]string) *ReceptorLogger {
	logger := NewReceptorLogger(prefix)
	logger.SetSuffix(suffix)

	return logger
}

// SetSuffix sets the suffix for the logger, overwriting any existing suffix.
func (rl *ReceptorLogger) SetSuffix(suffix map[string]string) {
	rl.m.Lock()
	defer rl.m.Unlock()
	rl.Suffix = suffix
}

// UpdateSuffix allows adding suffix key/value pairs to an existing suffix.
// If the key already exists, the value will be overwritten.
// If the suffix is nil, it will be initialized.
// This is useful for adding additional context to log messages when you want to keep the existing suffix key/value pairs.
func (rl *ReceptorLogger) UpdateSuffix(suffix map[string]string) {
	rl.m.Lock()
	defer rl.m.Unlock()
	if rl.Suffix == nil {
		rl.Suffix = make(map[string]string)
	}
	for k, v := range suffix {
		rl.Suffix[k] = v
	}
}

// SetOutput sets the output destination for the logger.
func (rl *ReceptorLogger) SetOutput(w io.Writer) {
	rl.Logger.SetOutput(w)
}

// SetShowTrace is a helper function for setting showTrace bool.
func (rl *ReceptorLogger) SetShowTrace(trace bool) {
	showTrace = trace
}

// GetLogLevel returns the log level.
func (rl *ReceptorLogger) GetLogLevel() int {
	return logLevel
}

// Error reports unexpected behavior, likely to result in termination.
func (rl *ReceptorLogger) Error(format string, v ...interface{}) {
	rl.Log(ErrorLevel, format, v...)
}

// SanError reports unexpected behavior, likely to result in termination.
func (rl *ReceptorLogger) SanitizedError(format string, v ...interface{}) {
	rl.SanitizedLog(ErrorLevel, format, v...)
}

// Warning reports unexpected behavior, not necessarily resulting in termination.
func (rl *ReceptorLogger) Warning(format string, v ...interface{}) {
	rl.Log(WarningLevel, format, v...)
}

// SanitizedWarning reports unexpected behavior, not necessarily resulting in termination.
func (rl *ReceptorLogger) SanitizedWarning(format string, v ...interface{}) {
	rl.SanitizedLog(WarningLevel, format, v...)
}

// Info provides general purpose statements useful to end user.
func (rl *ReceptorLogger) Info(format string, v ...interface{}) {
	rl.Log(InfoLevel, format, v...)
}

// SanitizedInfo provides general purpose statements useful to end user.
func (rl *ReceptorLogger) SanitizedInfo(format string, v ...interface{}) {
	rl.SanitizedLog(InfoLevel, format, v...)
}

// Debug contains extra information helpful to developers.
func (rl *ReceptorLogger) Debug(format string, v ...interface{}) {
	rl.Log(DebugLevel, format, v...)
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
	rl.Debug(fmt.Sprintf("PACKET TRACING ENABLED: %s%s%s", connectionTypeMessage, workunitIDMessage, payloadMessage)) //nolint:govet
}

// SanitizedDebug contains extra information helpful to developers.
func (rl *ReceptorLogger) SanitizedDebug(format string, v ...interface{}) {
	rl.SanitizedLog(DebugLevel, format, v...)
}

// Trace outputs detailed packet traversal.
func (rl *ReceptorLogger) Trace(format string, v ...interface{}) {
	if showTrace {
		rl.SetPrefix("TRACE")
		rl.Log(logLevel, format, v...)
	}
}

// SanitizedTrace outputs detailed packet traversal.
func (rl *ReceptorLogger) SanitizedTrace(format string, v ...interface{}) {
	if showTrace {
		rl.SetPrefix("TRACE")
		rl.SanitizedLog(logLevel, format, v...)
	}
}

// Log adds a prefix and prints a given log message.
func (rl *ReceptorLogger) Log(level int, format string, v ...interface{}) {
	if logger != nil {
		logger(level, format, v...)

		return
	}
	var prefix string
	logLevelName, err := LogLevelToName(level)
	if err != nil {
		rl.Error("Log entry received with invalid level: %s\n", fmt.Sprintf(format, v...))

		return
	}
	if rl.GetPrefix() != "" {
		prefix = rl.GetPrefix() + " " + strings.ToUpper(logLevelName) + " "
	} else {
		prefix = strings.ToUpper(logLevelName) + " "
	}

	if logLevel >= level {
		rl.Logger.SetPrefix(prefix)
		format, v = rl.appendSuffix(format, v)
		rl.Logger.Printf(format, v...)
	}
}

func (rl *ReceptorLogger) appendSuffix(format string, v []interface{}) (string, []interface{}) {
	if rl.Suffix != nil {
		rl.m.Lock()
		defer rl.m.Unlock()

		var b strings.Builder
		b.WriteString("{")
		first := true
		for k, val := range rl.Suffix {
			if !first {
				b.WriteString(",")
			}
			first = false
			b.WriteString(`"`)
			b.WriteString(k)
			b.WriteString(`":"`)
			b.WriteString(val)
			b.WriteString(`"`)
		}
		b.WriteString("}")

		format = strings.ReplaceAll(format, "\n", "") + " %s"
		v = append(v, b.String())
	}

	return format, v
}

// SanitizedLog adds a prefix and prints a given log message.
func (rl *ReceptorLogger) SanitizedLog(level int, format string, v ...interface{}) {
	if logger != nil {
		logger(level, format, v...)

		return
	}
	var prefix string
	logLevelName, err := LogLevelToName(level)
	if err != nil {
		message := fmt.Sprintf(format, v...)
		sanMessage := strings.ReplaceAll(message, "\n", "")
		rl.Error("Log entry received with invalid level: %s\n", sanMessage)

		return
	}
	if rl.GetPrefix() != "" {
		prefix = rl.GetPrefix() + " " + strings.ToUpper(logLevelName) + " "
	} else {
		prefix = strings.ToUpper(logLevelName) + " "
	}

	if logLevel >= level {
		message, v := rl.appendSuffix(format, v)
		message = fmt.Sprintf(message, v...)
		sanMessage := strings.ReplaceAll(message, "\n", "")
		rl.Logger.SetPrefix(prefix)
		rl.Logger.Print(sanMessage)
	}
}

func (rl *ReceptorLogger) SetPrefix(prefix string) {
	rl.m.Lock()
	defer rl.m.Unlock()
	rl.Prefix = prefix
}

func (rl *ReceptorLogger) GetPrefix() string {
	rl.m.Lock()
	defer rl.m.Unlock()

	return rl.Prefix
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
	Level string `description:"Log level: Error, Warning, Info or Debug" barevalue:"yes" default:"error"`
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
	return nil
}

func init() {
	version := viper.GetInt("version")
	if version > 1 {
		return
	}
	logLevel = InfoLevel
	showTrace = false
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime)

	cmdline.RegisterConfigTypeForApp("receptor-logging",
		"log-level", "Specifies the verbosity level for command output", LoglevelCfg{}, cmdline.Singleton)
	cmdline.RegisterConfigTypeForApp("receptor-logging",
		"trace", "Enables packet tracing output", TraceCfg{}, cmdline.Singleton)
}
