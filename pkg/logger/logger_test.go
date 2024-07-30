package logger_test

import (
	"fmt"
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
