package controlsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	gomock "go.uber.org/mock/gomock"
)

func CheckExpectedError(expectedError bool, errorMessage string, t *testing.T, err error) {
	if expectedError && errorMessage != err.Error() {
		t.Errorf("expected: %s , received: %s", errorMessage, err)
	}

	if !expectedError && err != nil {
		t.Error(err)
	}
}

func TestConnectInitFromString(t *testing.T) {
	connectCommandType := controlsvc.ConnectCommandType{}

	initFromStringTestCases := []struct {
		name          string
		expectedError bool
		errorMessage  string
		input         string
	}{
		{
			name:          "no connect target",
			expectedError: true,
			errorMessage:  "no connect target",
			input:         "",
		},
		{
			name:          "too many parameters",
			expectedError: true,
			errorMessage:  "too many parameters",
			input:         "one two three four",
		},
		{
			name:          "three params - pass",
			expectedError: false,
			errorMessage:  "",
			input:         "one two three",
		},
	}

	for _, testCase := range initFromStringTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := connectCommandType.InitFromString(testCase.input)

			CheckExpectedError(testCase.expectedError, testCase.errorMessage, t, err)
		})
	}
}

func TestConnectInitFromJSON(t *testing.T) {
	connectCommandType := controlsvc.ConnectCommandType{}

	initFromJSONTestCases := []struct {
		name          string
		expectedError bool
		errorMessage  string
		input         map[string]interface{}
	}{
		BuildInitFromJSONTestCases("no connect target node", true, "no connect target node", map[string]interface{}{}),
		BuildInitFromJSONTestCases("connect target node must be string 1", true, "connect target node must be string", map[string]interface{}{"node": 7}),
		BuildInitFromJSONTestCases("no connect target service", true, "no connect target service", map[string]interface{}{"node": "node1"}),
		BuildInitFromJSONTestCases("connect target service must be string1", true, "connect target service must be string", map[string]interface{}{"node": "node2", "service": 7}),
		BuildInitFromJSONTestCases("connect tls name be string", true, "connect tls name must be string", map[string]interface{}{"node": "node3", "service": "service1", "tls": 7}),
		BuildInitFromJSONTestCases("pass with empty tls config", false, "connect target service must be string", map[string]interface{}{"node": "node4", "service": "service2"}),
		BuildInitFromJSONTestCases("pass with all targets and tls config", false, "", map[string]interface{}{"node": "node4", "service": "service3", "tls": "tls1"}),
	}

	for _, testCase := range initFromJSONTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := connectCommandType.InitFromJSON(testCase.input)
			CheckExpectedError(testCase.expectedError, testCase.errorMessage, t, err)
		})
	}
}

func TestConnectControlFunc(t *testing.T) {
	connectCommand := controlsvc.ConnectCommand{}
	ctrl := gomock.NewController(t)
	mockNetceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	mockControlFunc := mock_controlsvc.NewMockControlFuncOperations(ctrl)
	logger := logger.NewReceptorLogger("")

	controlFuncTestCases := []struct {
		name          string
		expectedError bool
		errorMessage  string
		expectedCalls func()
	}{
		{
			name:          "tls config error",
			expectedError: true,
			errorMessage:  "terminated tls",
			expectedCalls: func() {
				mockNetceptor.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("terminated tls"))
			},
		},
		{
			name:          "dial error",
			errorMessage:  "terminated dial",
			expectedError: true,
			expectedCalls: func() {
				mockNetceptor.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockNetceptor.EXPECT().Dial(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("terminated dial"))
			},
		},
		{
			name:          "bridge conn error",
			errorMessage:  "terminated bridge conn",
			expectedError: true,
			expectedCalls: func() {
				mockNetceptor.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockNetceptor.EXPECT().Dial(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockControlFunc.EXPECT().BridgeConn(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("terminated bridge conn"))
				mockNetceptor.EXPECT().GetLogger().Return(logger)
			},
		},
		{
			name:          "control func pass",
			errorMessage:  "",
			expectedError: false,
			expectedCalls: func() {
				mockNetceptor.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockNetceptor.EXPECT().Dial(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockControlFunc.EXPECT().BridgeConn(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockNetceptor.EXPECT().GetLogger().Return(logger)
			},
		},
	}

	for _, testCase := range controlFuncTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()
			_, err := connectCommand.ControlFunc(context.Background(), mockNetceptor, mockControlFunc)

			CheckExpectedError(testCase.expectedError, testCase.errorMessage, t, err)
		})
	}
}
