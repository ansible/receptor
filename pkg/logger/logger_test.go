package logger_test

import (
	"bytes"
	"fmt"
	"os"
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
	logFilePath := "/tmp/test-output"
	logger.SetGlobalLogLevel(4)
	receptorLogger := logger.NewReceptorLogger("testDebugPayload")
	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		t.Error("error creating test-output file")
	}

	payload := "Testing debugPayload"
	workUnitID := "1234"
	connectionType := "unix socket"

	debugPayloadTestCases := []struct {
		name           string
		debugPayload   int
		payload        string
		workUnitID     string
		connectionType string
		expectedLogs   []string
	}{
		{name: "debugPayload no log", debugPayload: 0, payload: "", workUnitID: "", connectionType: "", expectedLogs: []string{}},
		{name: "debugPayload log level 1", debugPayload: 1, payload: "", workUnitID: "", connectionType: connectionType, expectedLogs: []string{fmt.Sprintf("Reading from %v", connectionType)}},
		{name: "debugPayload log level 2 with workUnitID", debugPayload: 2, payload: "", workUnitID: workUnitID, connectionType: connectionType, expectedLogs: []string{fmt.Sprintf("Reading from %v", connectionType), fmt.Sprintf("Work unit %v received command", workUnitID)}},
		{name: "debugPayload log level 2 without workUnitID", debugPayload: 2, payload: "", workUnitID: "", connectionType: connectionType, expectedLogs: []string{fmt.Sprintf("Reading from %v", connectionType)}},
		{name: "debugPayload log level 3 with workUnitID", debugPayload: 3, payload: payload, workUnitID: workUnitID, connectionType: connectionType, expectedLogs: []string{fmt.Sprintf("Reading from %v", connectionType), fmt.Sprintf("Work unit %v stdin: %v", workUnitID, payload)}},
		{name: "debugPayload log level 3 without workUnitID", debugPayload: 3, payload: payload, workUnitID: "", connectionType: connectionType, expectedLogs: []string{fmt.Sprintf("Reading from %v", connectionType), fmt.Sprintf("Response reading from conn: %v", payload)}},
	}

	for _, testCase := range debugPayloadTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			receptorLogger.SetOutput(logFile)
			receptorLogger.DebugPayload(testCase.debugPayload, testCase.payload, testCase.workUnitID, testCase.connectionType)

			testOutput, err := os.ReadFile(logFilePath)
			if err != nil {
				t.Error("error reading test-output file")
			}
			for _, expectedlog := range testCase.expectedLogs {
				if !bytes.Contains(testOutput, []byte(expectedlog)) {
					t.Errorf("failed to log correctly, expected: %v got %v", expectedlog, string(testOutput))
				}
			}
			if err := os.Truncate(logFilePath, 0); err != nil {
				t.Errorf("failed to truncate: %v", err)
			}
		})
	}
}
