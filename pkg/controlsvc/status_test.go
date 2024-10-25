package controlsvc_test

import (
	"context"
	"testing"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	"github.com/ansible/receptor/pkg/netceptor"
	"go.uber.org/mock/gomock"
)

func TestStatusInitFromString(t *testing.T) {
	statusCommandType := controlsvc.StatusCommandType{}

	initFromStringTestCases := []struct {
		name          string
		expectedError bool
		errorMessage  string
		input         string
	}{
		{
			name:          "status command does not take parameters",
			expectedError: true,
			errorMessage:  "status command does not take parameters",
			input:         "one",
		},
		{
			name:          "pass without params",
			expectedError: false,
			errorMessage:  "",
			input:         "",
		},
	}

	for _, testCase := range initFromStringTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := statusCommandType.InitFromString(testCase.input)

			CheckExpectedError(testCase.expectedError, testCase.errorMessage, t, err)
		})
	}
}

func TestStatusInitFromJSON(t *testing.T) {
	statusCommandType := controlsvc.StatusCommandType{}

	initFromJSONTestCases := []struct {
		name          string
		expectedError bool
		errorMessage  string
		input         map[string]interface{}
	}{
		{
			name:          "each element of requested_fields must be a string",
			expectedError: true,
			errorMessage:  "each element of requested_fields must be a string",
			input: map[string]interface{}{
				"requested_fields": []interface{}{
					0: 7,
				},
			},
		},
		{
			name:          "pass with no requested fields",
			expectedError: false,
			errorMessage:  "",
			input:         map[string]interface{}{},
		},
		{
			name:          "pass with requested fields",
			expectedError: false,
			errorMessage:  "",
			input: map[string]interface{}{
				"requested_fields": []interface{}{
					0: "request",
				},
			},
		},
	}

	for _, testCase := range initFromJSONTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := statusCommandType.InitFromJSON(testCase.input)

			CheckExpectedError(testCase.expectedError, testCase.errorMessage, t, err)
		})
	}
}

func TestStatusControlFunc(t *testing.T) {
	statusCommand := controlsvc.StatusCommand{}
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
			name:          "control func pass",
			errorMessage:  "",
			expectedError: false,
			expectedCalls: func() {
				mockNetceptor.EXPECT().Status().Return(netceptor.Status{})
			},
		},
	}

	for _, testCase := range controlFuncTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()

			_, err := statusCommand.ControlFunc(context.Background(), mockNetceptor, mockControlFunc)

			if !testCase.expectedError && err != nil {
				t.Error(err)
			}
		})
	}
}
