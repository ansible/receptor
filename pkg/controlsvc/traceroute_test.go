package controlsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/golang/mock/gomock"
)

func TestTracerouteInitFromString(t *testing.T) {
	tracerouteCommandType := controlsvc.TracerouteCommandType{}

	initFromStringTestCases := []struct {
		name          string
		expectedError bool
		errorMessage  string
		input         string
	}{
		{
			name:          "no traceroute target 1",
			expectedError: true,
			errorMessage:  "no traceroute target",
			input:         "",
		},
		{
			name:          "pass with params",
			expectedError: false,
			errorMessage:  "",
			input:         "one",
		},
	}

	for _, testCase := range initFromStringTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := tracerouteCommandType.InitFromString(testCase.input)

			CheckExpectedError(testCase.expectedError, testCase.errorMessage, t, err)
		})
	}
}

func TestTracerouteInitFromJSON(t *testing.T) {
	tracerouteCommandType := controlsvc.TracerouteCommandType{}

	initFromJSONTestCases := []InitFromJSONTestCase{
		BuildInitFromJSONTestCases("no traceroute target 2", true, "no traceroute target", map[string]interface{}{}),
		BuildInitFromJSONTestCases("traceroute target must be string", true, "traceroute target must be string", map[string]interface{}{"target": 7}),
		BuildInitFromJSONTestCases("pass with target", false, "", map[string]interface{}{"target": "some target"}),
	}

	for _, testCase := range initFromJSONTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := tracerouteCommandType.InitFromJSON(testCase.input)

			CheckExpectedError(testCase.expectedError, testCase.errorMessage, t, err)
		})
	}
}

func TestTracerouteControlFunc(t *testing.T) {
	tracerouteCommand := controlsvc.TracerouteCommand{}
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
			name:          "control func pass with result error",
			errorMessage:  "terminated",
			expectedError: false,
			expectedCalls: func() {
				c := make(chan *netceptor.TracerouteResult)

				go func() {
					c <- &netceptor.TracerouteResult{
						Err: errors.New("terminated"),
					}
					close(c)
				}()
				mockNetceptor.EXPECT().Traceroute(gomock.Any(), gomock.Any()).Return(c)
			},
		},
		{
			name:          "control func pass",
			errorMessage:  "",
			expectedError: false,
			expectedCalls: func() {
				c := make(chan *netceptor.TracerouteResult)

				go func() {
					c <- &netceptor.TracerouteResult{}
					close(c)
				}()
				mockNetceptor.EXPECT().Traceroute(gomock.Any(), gomock.Any()).Return(c)
			},
		},
	}

	for _, testCase := range controlFuncTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.expectedCalls()

			cfr, _ := tracerouteCommand.ControlFunc(context.Background(), mockNetceptor, mockControlFunc)
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
