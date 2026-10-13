package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ghjm/cmdline"
	"github.com/spf13/viper"
)

var logLevel atomic.Int32

// logTimestampFormat matches the date/time layout produced by log.LstdFlags.
// The underlying log.Logger is created without flags so that the timestamp can
// be placed after the level label rather than before it.
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
	logLevel.Store(0)
}

// SetGlobalLogLevel is a helper function for setting the global log level.
func SetGlobalLogLevel(level int) {
	logLevel.Store(int32(level)) //nolint:gosec // G115: log levels are small constants (0-5)
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
	return int(logLevel.Load())
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
	logger *log.Logger
	// suffix holds structured fields rendered after each log line. It is never
	// written after construction, so it is safe to read from concurrent
	// goroutines without synchronization.
	suffix map[string]string
}

// NewReceptorLogger instantiates a logger with no structured suffix. Use
// WithExtraSuffix to attach fields such as node_id or remote_id.
func NewReceptorLogger() *ReceptorLogger {
	return &ReceptorLogger{
		logger: log.New(os.Stdout, "", 0),
	}
}

// WithSuffix returns a new logger that shares rl's underlying log.Logger --
// and therefore its writer and write mutex -- and carries the given structured
// suffix. The receiver is not modified; the new logger treats suffix as
// read-only afterwards, so it can be shared safely by concurrent goroutines.
// Because the sink is shared, SetOutput on any logger in the family affects all
// of them.
func (rl *ReceptorLogger) WithSuffix(suffix map[string]string) *ReceptorLogger {
	return &ReceptorLogger{
		logger: rl.logger,
		suffix: suffix,
	}
}

// WithExtraSuffix returns a new logger that shares rl's underlying log.Logger
// and carries rl's structured suffix merged with the given fields. The
// receiver and rl's suffix map are not modified.
func (rl *ReceptorLogger) WithExtraSuffix(fields map[string]string) *ReceptorLogger {
	merged := maps.Clone(rl.suffix)
	if merged == nil {
		merged = make(map[string]string, len(fields))
	}
	maps.Copy(merged, fields)

	return rl.WithSuffix(merged)
}

// SetOutput sets the output destination for the logger.
func (rl *ReceptorLogger) SetOutput(w io.Writer) {
	rl.logger.SetOutput(w)
}

// SetShowTrace enables trace-level logging.
func (rl *ReceptorLogger) SetShowTrace(trace bool) {
	if trace {
		SetGlobalLogLevel(TraceLevel)
	}
}

// GetLogLevel returns the log level.
func (rl *ReceptorLogger) GetLogLevel() int {
	return int(logLevel.Load())
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

// output writes a formatted log line.
func (rl *ReceptorLogger) output(levelName, msg string) {
	now := time.Now().Format(logTimestampFormat)
	rl.logger.Print(strings.ToUpper(levelName) + " " + now + " " + msg)
}

// log prepends level and timestamp then prints a given log message.
func (rl *ReceptorLogger) log(level int, format string, v ...interface{}) {
	if logger != nil {
		logger(level, format, v...)

		return
	}

	rl.emit(level, false, format, v...)
}

// sanitizedLog prepends level and timestamp, strips embedded newlines, then
// prints a given log message.
func (rl *ReceptorLogger) sanitizedLog(level int, format string, v ...interface{}) {
	if logger != nil {
		logger(level, format, v...)

		return
	}

	rl.emit(level, true, format, v...)
}

// emit formats and writes a single log line at the given level.
func (rl *ReceptorLogger) emit(level int, sanitize bool, format string, v ...interface{}) {
	logLevelName, err := LogLevelToName(level)
	if err != nil {
		sanitized := strings.ReplaceAll(fmt.Sprintf(format, v...), "\n", "")
		rl.Error("Log entry received with invalid level: %s\n", sanitized)

		return
	}

	if int(logLevel.Load()) < level {
		return
	}

	msg := fmt.Sprintf(format, v...)
	if sanitize {
		msg = strings.ReplaceAll(msg, "\n", "")
	}
	rl.output(logLevelName, rl.appendSuffix(msg))
}

// appendSuffix appends the logger's structured suffix to msg.
func (rl *ReceptorLogger) appendSuffix(msg string) string {
	if len(rl.suffix) == 0 {
		return msg
	}
	b, err := json.Marshal(rl.suffix)
	if err != nil {
		return msg
	}

	return strings.ReplaceAll(msg, "\n", "") + " " + string(b)
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
	logLevel.Store(InfoLevel)
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime)

	cmdline.RegisterConfigTypeForApp("receptor-logging",
		"log-level", "Specifies the verbosity level for command output", LoglevelCfg{}, cmdline.Singleton)
	cmdline.RegisterConfigTypeForApp("receptor-logging",
		"trace", "Enables packet tracing output", TraceCfg{}, cmdline.Singleton)
}
