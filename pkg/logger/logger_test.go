package logger

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestGetLogLevelByName(t *testing.T) {
	receptorLogger := NewReceptorLogger("")

	testCases := []struct {
		name  string
		error bool
	}{
		{name: "error"},
		{name: "warning"},
		{name: "info"},
		{name: "debug"},
		{name: "trace"},
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
	receptorLogger := NewReceptorLogger("")
	_, err := receptorLogger.GetLogLevelByName("does not exist")
	if err == nil {
		t.Error("should have error")
	}
}

func TestLogLevelToName(t *testing.T) {
	receptorLogger := NewReceptorLogger("")

	testCases := []struct {
		level int
	}{
		{level: 1},
		{level: 2},
		{level: 3},
		{level: 4},
		{level: 5},
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
	receptorLogger := NewReceptorLogger("")
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
		{"error", "error", ErrorLevel, false},
		{"warning", "warning", WarningLevel, false},
		{"info", "info", InfoLevel, false},
		{"debug", "DEBUG", DebugLevel, false},
		{"trace", "trace", TraceLevel, false},
		{"invalid", "garbage", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := LoglevelCfg{Level: tt.level}
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
			got := GetLogLevel()
			if got != tt.wantLevel {
				t.Errorf("expected log level %d, got %d", tt.wantLevel, got)
			}
			SetGlobalLogLevel(InfoLevel) // Reset to default after test
		})
	}
}

func TestSanitizedLog(t *testing.T) {
	// Save and restore global logger and log level
	origLogger := GetLogLevel()
	defer SetGlobalLogLevel(origLogger)
	SetGlobalLogLevel(DebugLevel)

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
			level:                 InfoLevel,
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
			level:                 InfoLevel,
			prefix:                "",
			format:                "noprefix %s",
			args:                  []interface{}{"ok"},
			expectContains:        []string{"INFO", "noprefix ok"},
			expectNoNewlineInside: false,
			registerLogger:        false,
		},
		{
			name:                  "logger function registered",
			level:                 InfoLevel,
			prefix:                "testPrefix",
			format:                "delegated %d",
			args:                  []interface{}{1},
			expectContains:        []string{},
			expectNoNewlineInside: false,
			registerLogger:        true,
			loggerCheck: func(t *testing.T, level int, format string, v ...interface{}) {
				if level != InfoLevel || format != "delegated %d" || v[0].(int) != 1 {
					t.Errorf("logger delegate called with wrong args: %v %v %v", level, format, v)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			oldLogger := GetRegisteredLogger()
			defer RegisterLogger(oldLogger)
			var called bool
			if tc.registerLogger {
				RegisterLogger(func(level int, format string, v ...interface{}) {
					called = true
					if tc.loggerCheck != nil {
						tc.loggerCheck(t, level, format, v...)
					}
				})
			} else {
				RegisterLogger(nil)
			}

			var buf bytes.Buffer
			rl := NewReceptorLogger(tc.prefix)
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
	SetGlobalLogLevel(4)
	receptorLogger := NewReceptorLogger("testDebugPayload")
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

func TestGetLogLevel(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)

	for _, level := range []int{ErrorLevel, WarningLevel, InfoLevel, DebugLevel, TraceLevel} {
		SetGlobalLogLevel(level)
		if got := GetLogLevel(); got != level {
			t.Errorf("GetLogLevel() = %d, want %d", got, level)
		}
	}
}

func TestLogMethods(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)

	origLogger := GetRegisteredLogger()
	defer RegisterLogger(origLogger)
	RegisterLogger(nil)

	type logCall func(rl *ReceptorLogger, msg string)

	cases := []struct {
		name      string
		minLevel  int
		levelName string
		call      logCall
	}{
		{"Warning", WarningLevel, "WARNING", func(rl *ReceptorLogger, msg string) { rl.Warning("%s", msg) }},
		{"Info", InfoLevel, "INFO", func(rl *ReceptorLogger, msg string) { rl.Info("%s", msg) }},
		{"Debug", DebugLevel, "DEBUG", func(rl *ReceptorLogger, msg string) { rl.Debug("%s", msg) }},
		{"Trace", TraceLevel, "TRACE", func(rl *ReceptorLogger, msg string) { rl.Trace("%s", msg) }},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/emits at level", func(t *testing.T) {
			SetGlobalLogLevel(tc.minLevel)
			var buf bytes.Buffer
			rl := NewReceptorLogger("")
			rl.SetOutput(&buf)
			tc.call(rl, "hello")
			got := buf.String()
			if !strings.Contains(got, tc.levelName) {
				t.Errorf("expected %q in output, got: %q", tc.levelName, got)
			}
			if !strings.Contains(got, "hello") {
				t.Errorf("expected message body in output, got: %q", got)
			}
		})

		t.Run(tc.name+"/suppressed below level", func(t *testing.T) {
			SetGlobalLogLevel(tc.minLevel - 1)
			var buf bytes.Buffer
			rl := NewReceptorLogger("")
			rl.SetOutput(&buf)
			tc.call(rl, "hello")
			if buf.Len() != 0 {
				t.Errorf("expected no output below level, got: %q", buf.String())
			}
		})
	}
}

func TestSanitizedLogMethods(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)

	origLogger := GetRegisteredLogger()
	defer RegisterLogger(origLogger)
	RegisterLogger(nil)

	type logCall func(rl *ReceptorLogger, msg string)

	cases := []struct {
		name      string
		minLevel  int
		levelName string
		call      logCall
	}{
		{"SanitizedWarning", WarningLevel, "WARNING", func(rl *ReceptorLogger, msg string) { rl.SanitizedWarning("%s", msg) }},
		{"SanitizedInfo", InfoLevel, "INFO", func(rl *ReceptorLogger, msg string) { rl.SanitizedInfo("%s", msg) }},
		{"SanitizedDebug", DebugLevel, "DEBUG", func(rl *ReceptorLogger, msg string) { rl.SanitizedDebug("%s", msg) }},
		{"SanitizedTrace", TraceLevel, "TRACE", func(rl *ReceptorLogger, msg string) { rl.SanitizedTrace("%s", msg) }},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/emits at level", func(t *testing.T) {
			SetGlobalLogLevel(tc.minLevel)
			var buf bytes.Buffer
			rl := NewReceptorLogger("")
			rl.SetOutput(&buf)
			tc.call(rl, "hello")
			got := buf.String()
			if !strings.Contains(got, tc.levelName) {
				t.Errorf("expected %q in output, got: %q", tc.levelName, got)
			}
			if !strings.Contains(got, "hello") {
				t.Errorf("expected message body in output, got: %q", got)
			}
		})

		t.Run(tc.name+"/suppressed below level", func(t *testing.T) {
			SetGlobalLogLevel(tc.minLevel - 1)
			var buf bytes.Buffer
			rl := NewReceptorLogger("")
			rl.SetOutput(&buf)
			tc.call(rl, "hello")
			if buf.Len() != 0 {
				t.Errorf("expected no output below level, got: %q", buf.String())
			}
		})

		t.Run(tc.name+"/strips embedded newlines", func(t *testing.T) {
			SetGlobalLogLevel(tc.minLevel)
			var buf bytes.Buffer
			rl := NewReceptorLogger("")
			rl.SetOutput(&buf)
			tc.call(rl, "line1\nline2")
			got := strings.TrimSuffix(buf.String(), "\n")
			if strings.Contains(got, "\n") {
				t.Errorf("expected embedded newlines stripped, got: %q", buf.String())
			}
		})
	}
}

func TestSetShowTrace(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)

	SetGlobalLogLevel(InfoLevel)
	rl := NewReceptorLogger("")
	rl.SetShowTrace(true)
	if GetLogLevel() != TraceLevel {
		t.Errorf("expected TraceLevel after SetShowTrace(true), got %d", GetLogLevel())
	}
}

func TestTraceCfgPrepare(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)

	cfg := TraceCfg{}
	if err := cfg.Prepare(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if GetLogLevel() != TraceLevel {
		t.Errorf("expected TraceLevel after TraceCfg.Prepare(), got %d", GetLogLevel())
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
	SetGlobalLogLevel(4)

	t.Run("initial suffix", func(t *testing.T) {
		var logBuffer bytes.Buffer
		testname := "Initial Suffix Example"
		suffix := map[string]string{
			"node_id":   "controller",
			"remote_id": "hop",
		}
		receptorLogger := NewReceptorLoggerWithSuffix("", suffix)
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
		receptorLogger := NewReceptorLoggerWithSuffix("", suffix)
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
