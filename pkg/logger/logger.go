package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/ghjm/cmdline"
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
func QuietMode() {
	logLevel = 0
}

// SetLogLevel is a helper function for setting logLevel int.
func SetLogLevel(level int) {
	logLevel = level
}

// SetOutput sets the output destination for the logger.
func SetOutput(w io.Writer) {
	log.SetOutput(w)
}

// SetShowTrace is a helper function for setting showTrace bool.
func SetShowTrace(trace bool) {
	showTrace = trace
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
}

type MessageFunc func(level int, format string, v ...interface{})

var logger MessageFunc

// Log sends a log message at a given level.
func Log(level int, format string, v ...interface{}) {
	if logger != nil {
		logger(level, format, v...)

		return
	}
	var prefix string
	logLevelName, err := LogLevelToName(level)
	if err != nil {
		Error("Log entry received with invalid level: %s\n", fmt.Sprintf(format, v...))

		return
	}
	prefix = strings.ToUpper(logLevelName) + " "
	if logLevel >= level {
		log.SetPrefix(prefix)
		log.Printf(format, v...)
	}
}

// RegisterLogger registers a function for log delivery.
func RegisterLogger(msgFunc MessageFunc) {
	logger = msgFunc
}

// Error reports unexpected behavior, likely to result in termination.
func Error(format string, v ...interface{}) {
	Log(ErrorLevel, format, v...)
}

// Warning reports unexpected behavior, not necessarily resulting in termination.
func Warning(format string, v ...interface{}) {
	Log(WarningLevel, format, v...)
}

// Info provides general purpose statements useful to end user.
func Info(format string, v ...interface{}) {
	Log(InfoLevel, format, v...)
}

// Debug contains extra information helpful to developers.
func Debug(format string, v ...interface{}) {
	Log(DebugLevel, format, v...)
}

// Trace outputs detailed packet traversal.
func Trace(format string, v ...interface{}) {
	if showTrace {
		log.SetPrefix("TRACE ")
		log.Printf(format, v...)
	}
}

type ReceptorLogger struct {
	log.Logger
	Prefix string
}

// NewReceptorLogger to instantiate a new logger object.
func NewReceptorLogger(prefix string) *ReceptorLogger {
	return &ReceptorLogger{
		Logger: *log.New(os.Stdout, prefix, log.LstdFlags),
		Prefix: prefix,
	}
}

// GetLogLevel returns the log level.
func (rl *ReceptorLogger) GetLogLevel() int {
	return logLevel
}

// Error reports unexpected behavior, likely to result in termination.
func (rl *ReceptorLogger) Error(format string, v ...interface{}) {
	rl.Log(ErrorLevel, format, v...)
}

// Warning reports unexpected behavior, not necessarily resulting in termination.
func (rl *ReceptorLogger) Warning(format string, v ...interface{}) {
	rl.Log(WarningLevel, format, v...)
}

// Info provides general purpose statements useful to end user.
func (rl *ReceptorLogger) Info(format string, v ...interface{}) {
	rl.Log(InfoLevel, format, v...)
}

// Debug contains extra information helpful to developers.
func (rl *ReceptorLogger) Debug(format string, v ...interface{}) {
	rl.Log(DebugLevel, format, v...)
}

// Trace outputs detailed packet traversal.
func (rl *ReceptorLogger) Trace(format string, v ...interface{}) {
	if showTrace {
		log.SetPrefix("TRACE ")
		log.Printf(format, v...)
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
		Error("Log entry received with invalid level: %s\n", fmt.Sprintf(format, v...))

		return
	}
	if rl.Prefix != "" {
		prefix = strings.ToUpper(logLevelName) + " " + rl.Prefix + " "
	} else {
		prefix = strings.ToUpper(logLevelName) + " "
	}

	if logLevel >= level {
		rl.Logger.SetPrefix(prefix)
		rl.Logger.Printf(format, v...)
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

type loglevelCfg struct {
	Level string `description:"Log level: Error, Warning, Info or Debug" barevalue:"yes" default:"error"`
}

func (cfg loglevelCfg) Init() error {
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

	cmdline.RegisterConfigTypeForApp("receptor-logging",
		"log-level", "Set specific log level output", loglevelCfg{}, cmdline.Singleton)
	cmdline.RegisterConfigTypeForApp("receptor-logging",
		"trace", "Enables packet tracing output", traceCfg{}, cmdline.Singleton)
}
