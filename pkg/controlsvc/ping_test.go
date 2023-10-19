package controlsvc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	gomock "go.uber.org/mock/gomock"
)

func TestPingInitFromString(t *testing.T) {
	pingCommandType := controlsvc.PingCommandType{}

	initFromStringTestCases := []struct {
		name          string
		expectedError bool
		errorMessage  string
		input         string
	}{
		{
			name:          "no ping target 1",
			expectedError: true,
			errorMessage:  "no ping target",
			input:         "",
		},
		{
			name:          "single param - pass",
			expectedError: false,
			errorMessage:  "",
			input:         "one",
		},
	}

	for _, testCase := range initFromStringTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := pingCommandType.InitFromString(testCase.input)

			CheckExpectedError(testCase.expectedError, testCase.errorMessage, t, err)
		})
	}
}

type InitFromJSONTestCase struct {
	name          string
	expectedError bool
	errorMessage  string
	input         map[string]interface{}
}

func BuildInitFromJSONTestCases(name string, expectedError bool, errorMessage string, input map[string]interface{}) InitFromJSONTestCase {
	return InitFromJSONTestCase{
		name:          name,
		expectedError: expectedError,
		errorMessage:  errorMessage,
		input:         input,
	}
}

func TestPingInitFromJSON(t *testing.T) {
	pingCommandType := controlsvc.PingCommandType{}

	initFromJSONTestCases := []InitFromJSONTestCase{
		BuildInitFromJSONTestCases("no ping target 2", true, "no ping target", map[string]interface{}{}),
		BuildInitFromJSONTestCases("ping target must be string", true, "ping target must be string", map[string]interface{}{"target": 7}),
		BuildInitFromJSONTestCases("three params - pass", false, "", map[string]interface{}{"target": "some target"}),
	}

	for _, testCase := range initFromJSONTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := pingCommandType.InitFromJSON(testCase.input)

			CheckExpectedError(testCase.expectedError, testCase.errorMessage, t, err)
		})
	}
}

func TestPingControlFunc(t *testing.T) {
	pingCommand := controlsvc.PingCommand{}
	ctrl := gomock.NewController(t)
	mockNetceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	mockControlFunc := mock_controlsvc.NewMockControlFuncOperations(ctrl)

	controlFuncTestCases := []struct {
		name          string
		expectedError bool
		errorMessage  string
		expectedCalls func()
	}{
		{
			name:          "ping error",
			expectedError: true,
			errorMessage:  "terminated ping",
			expectedCalls: func() {
				mockNetceptor.EXPECT().Ping(gomock.Any(), gomock.Any(), gomock.Any()).Return(time.Second, "", errors.New("terminated ping"))
				mockNetceptor.EXPECT().MaxForwardingHops()
			},
		},
		{
			name:          "control func pass",
			errorMessage:  "",
			expectedError: false,
			expectedCalls: func() {
				mockNetceptor.EXPECT().Ping(gomock.Any(), gomock.Any(), gomock.Any()).Return(time.Second, "", nil)
				mockNetceptor.EXPECT().MaxForwardingHops()
			},
		},
	}

	for _, testCase := range controlFuncTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()

			cfr, _ := pingCommand.ControlFunc(context.Background(), mockNetceptor, mockControlFunc)
			err, ok := cfr["Error"]

			if testCase.expectedError && testCase.errorMessage != err {
				t.Errorf("expected: %s , received: %s", testCase.errorMessage, err)
			}

			if !testCase.expectedError && ok {
				t.Error(cfr["Error"])
			}
		})
	}
}
