package logger_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
)

func TestGetLogLevelByName(t *testing.T) {
	receptorLogger := logger.NewReceptorLogger("")

	testCases := []struct {
		name  string
		error bool
	}{
		{name: "error"},
		{name: "warning"},
		{name: "info"},
		{name: "debug"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := receptorLogger.GetLogLevelByName(testCase.name)
			if err != nil {
				t.Error(err)
			}
		})
	}
}

func TestGetLogLevelByNameWithError(t *testing.T) {
	receptorLogger := logger.NewReceptorLogger("")
	_, err := receptorLogger.GetLogLevelByName("does not exist")
	if err == nil {
		t.Error("should have error")
	}
}

func TestLogLevelToName(t *testing.T) {
	receptorLogger := logger.NewReceptorLogger("")

	testCases := []struct {
		level int
	}{
		{level: 1},
		{level: 2},
		{level: 3},
		{level: 4},
	}

	for _, testCase := range testCases {
		name := fmt.Sprintf("level: %d", testCase.level)
		t.Run(name, func(t *testing.T) {
			_, err := receptorLogger.LogLevelToName(testCase.level)
			if err != nil {
				t.Error(err)
			}
		})
	}
}

func TestLogLevelToNameWithError(t *testing.T) {
	receptorLogger := logger.NewReceptorLogger("")
	_, err := receptorLogger.LogLevelToName(123)
	if err == nil {
		t.Error("should have error")
	}
}

func TestLoglevelCfgInit(t *testing.T) {
	testCases := []struct {
		name      string
		level     string
		wantLevel int
		wantErr   bool
	}{
		{"error", "error", logger.ErrorLevel, false},
		{"warning", "warning", logger.WarningLevel, false},
		{"info", "info", logger.InfoLevel, false},
		{"debug", "DEBUG", logger.DebugLevel, false},
		{"invalid", "garbage", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := logger.LoglevelCfg{Level: tt.level}
			err := cfg.Init()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for level %q, got nil", tt.level)
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error for level %q: %v", tt.level, err)
			}
			got := logger.GetLogLevel()
			if got != tt.wantLevel {
				t.Errorf("expected log level %d, got %d", tt.wantLevel, got)
			}
			logger.SetGlobalLogLevel(logger.InfoLevel) // Reset to default after test
		})
	}
}

func TestSanitizedLog(t *testing.T) {
	// Save and restore global logger and log level
	origLogger := logger.GetLogLevel()
	defer logger.SetGlobalLogLevel(origLogger)
	logger.SetGlobalLogLevel(logger.DebugLevel)

	testCases := []struct {
		name                  string
		level                 int
		prefix                string
		format                string
		args                  []interface{}
		expectContains        []string
		expectNoNewlineInside bool
		registerLogger        bool
		loggerCheck           func(t *testing.T, level int, format string, v ...interface{})
	}{
		{
			name:                  "valid level with prefix and newline",
			level:                 logger.InfoLevel,
			prefix:                "testPrefix",
			format:                "hello\nworld %d",
			args:                  []interface{}{42},
			expectContains:        []string{"testPrefix INFO", "helloworld 42"},
			expectNoNewlineInside: true,
			registerLogger:        false,
		},
		{
			name:                  "invalid log level triggers error",
			level:                 9999,
			prefix:                "testPrefix",
			format:                "bad\nmessage %d",
			args:                  []interface{}{7},
			expectContains:        []string{"Log entry received with invalid level: badmessage 7"},
			expectNoNewlineInside: true,
			registerLogger:        false,
		},
		{
			name:                  "no prefix",
			level:                 logger.InfoLevel,
			prefix:                "",
			format:                "noprefix %s",
			args:                  []interface{}{"ok"},
			expectContains:        []string{"INFO", "noprefix ok"},
			expectNoNewlineInside: false,
			registerLogger:        false,
		},
		{
			name:                  "logger function registered",
			level:                 logger.InfoLevel,
			prefix:                "testPrefix",
			format:                "delegated %d",
			args:                  []interface{}{1},
			expectContains:        []string{},
			expectNoNewlineInside: false,
			registerLogger:        true,
			loggerCheck: func(t *testing.T, level int, format string, v ...interface{}) {
				if level != logger.InfoLevel || format != "delegated %d" || v[0].(int) != 1 {
					t.Errorf("logger delegate called with wrong args: %v %v %v", level, format, v)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			oldLogger := logger.GetRegisteredLogger()
			defer logger.RegisterLogger(oldLogger)
			var called bool
			if tc.registerLogger {
				logger.RegisterLogger(func(level int, format string, v ...interface{}) {
					called = true
					if tc.loggerCheck != nil {
						tc.loggerCheck(t, level, format, v...)
					}
				})
			} else {
				logger.RegisterLogger(nil)
			}

			var buf bytes.Buffer
			rl := logger.NewReceptorLogger(tc.prefix)
			rl.SetOutput(&buf)

			rl.SanitizedLog(tc.level, tc.format, tc.args...)

			got := buf.String()

			for _, substr := range tc.expectContains {
				if !strings.Contains(got, substr) {
					t.Errorf("expected log output to contain %q, got: %q", substr, got)
				}
			}

			if tc.expectNoNewlineInside {
				if strings.Contains(strings.TrimSuffix(got, "\n"), "\n") {
					t.Errorf("expected embedded newlines to be removed from log message, got: %q", got)
				}
			}

			if tc.registerLogger && !called {
				t.Error("expected registered logger function to be called")
			}
		})
	}
}

func TestDebugPayload(t *testing.T) {
	var logBuffer bytes.Buffer
	logger.SetGlobalLogLevel(4)
	receptorLogger := logger.NewReceptorLogger("testDebugPayload")
	receptorLogger.SetOutput(&logBuffer)
	payload := "Testing debugPayload"
	workUnitID := "1234"
	connectionType := "unix socket"

	debugPayloadTestCases := []struct {
		name           string
		debugPayload   int
		payload        string
		workUnitID     string
		connectionType string
		expectedLog    string
	}{
		{name: "debugPayload no log", debugPayload: 0, payload: "", workUnitID: "", connectionType: "", expectedLog: ""},
		{name: "debugPayload log level 1", debugPayload: 1, payload: "", workUnitID: "", connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v", connectionType)},
		{name: "debugPayload log level 2 with workUnitID", debugPayload: 2, payload: "", workUnitID: workUnitID, connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v with work unit %v", connectionType, workUnitID)},
		{name: "debugPayload log level 2 without workUnitID", debugPayload: 2, payload: "", workUnitID: "", connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v", connectionType)},
		{name: "debugPayload log level 3 with workUnitID", debugPayload: 3, payload: payload, workUnitID: workUnitID, connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v with work unit %v with a payload of: %v", connectionType, workUnitID, payload)},
		{name: "debugPayload log level 3 without workUnitID", debugPayload: 3, payload: payload, workUnitID: "", connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v, work unit not created yet with a payload of: %v", connectionType, payload)},
		{name: "debugPayload log level 3 without workUnitID and payload is new line", debugPayload: 3, payload: "\n", workUnitID: "", connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v, work unit not created yet with a payload of: %v", connectionType, "\n")},
		{name: "debugPayload log level 3 without workUnitID or payload", debugPayload: 3, payload: "", workUnitID: "", connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v, work unit not created yet with a payload of: %v", connectionType, "")},
	}

	for _, testCase := range debugPayloadTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			receptorLogger.DebugPayload(testCase.debugPayload, testCase.payload, testCase.workUnitID, testCase.connectionType)
			testOutput := logBuffer.Bytes()

			if !strings.Contains(string(testOutput), testCase.expectedLog) {
				t.Errorf("failed to log correctly, expected: %v got %v", testCase.expectedLog, string(testOutput))
			}
			logBuffer.Reset()
		})
	}
}

func assertSuffixFieldsPresent(t *testing.T, logLine string, expected map[string]string) {
	t.Helper()
	for k, v := range expected {
		needle := `"` + k + `":"` + v + `"`
		if !strings.Contains(logLine, needle) {
			t.Errorf("expected key-value pair %s not found in log: %s", needle, logLine)
		}
	}
}

func TestGetLoggerWithSuffix(t *testing.T) {
	logger.SetGlobalLogLevel(4)

	t.Run("initial suffix", func(t *testing.T) {
		var logBuffer bytes.Buffer
		testname := "Initial Suffix Example"
		suffix := map[string]string{
			"node_id":   "controller",
			"remote_id": "hop",
		}
		receptorLogger := logger.NewReceptorLoggerWithSuffix("", suffix)
		receptorLogger.SetOutput(&logBuffer)

		receptorLogger.Error("%s", testname)
		if !strings.Contains(logBuffer.String(), testname) {
			t.Errorf("expected log message %s not found in log: %s", testname, logBuffer.String())
		}
		assertSuffixFieldsPresent(t, logBuffer.String(), suffix)
	})
	t.Run("updated suffix", func(t *testing.T) {
		var logBuffer bytes.Buffer
		testname := "Updated Suffix Example"
		suffix := map[string]string{
			"node_id":   "controller",
			"remote_id": "hop",
		}
		receptorLogger := logger.NewReceptorLoggerWithSuffix("", suffix)
		receptorLogger.SetOutput(&logBuffer)

		updated := map[string]string{
			"cost": "12",
		}
		receptorLogger.UpdateSuffix(updated)
		receptorLogger.SanitizedError("%s", testname)

		if !strings.Contains(logBuffer.String(), testname) {
			t.Errorf("expected log message %s not found in log: %s", testname, logBuffer.String())
		}

		assertSuffixFieldsPresent(t, logBuffer.String(), suffix)
		assertSuffixFieldsPresent(t, logBuffer.String(), updated)
	})
}
