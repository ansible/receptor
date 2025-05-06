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
