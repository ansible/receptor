package workceptor_test

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"github.com/ansible/receptor/tests/utils"
	"github.com/golang/mock/gomock"
)

func testSetup(t *testing.T) (*gomock.Controller, *mock_workceptor.MockNetceptorForWorkceptor, *workceptor.Workceptor) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
	mockNetceptor.EXPECT().NodeID().Return("test")

	logger := logger.NewReceptorLogger("")
	mockNetceptor.EXPECT().GetLogger().AnyTimes().Return(logger)

	w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
	if err != nil {
		t.Errorf("Error while creating Workceptor: %v", err)
	}

	return ctrl, mockNetceptor, w
}

func TestAllocateUnit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWorkUnit := mock_workceptor.NewMockWorkUnit(ctrl)
	ctx := context.Background()
	mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)

	logger := logger.NewReceptorLogger("")
	mockNetceptor.EXPECT().GetLogger().AnyTimes().Return(logger)

	workFunc := func(bwu workceptor.BaseWorkUnitForWorkUnit, w *workceptor.Workceptor, unitID string, workType string) workceptor.WorkUnit { //nolint:revive
		return mockWorkUnit
	}

	mockNetceptor.EXPECT().NodeID().Return("test")
	w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
	if err != nil {
		t.Errorf("Error while creating Workceptor: %v", err)
	}
	const testType = "testType"

	mockNetceptor.EXPECT().AddWorkCommand(gomock.Any(), gomock.Any()).Return(nil)
	w.RegisterWorker(testType, workFunc, false)

	const paramError = "SetFromParams error"
	const saveError = "Save error"
	testCases := []struct {
		name               string
		workType           string
		setFromParamsError error
		saveError          error
		mockSetParam       bool
		mockSave           bool
		expectedError      string
	}{
		{
			name:               "normal case",
			workType:           testType,
			setFromParamsError: nil,
			saveError:          nil,
			mockSetParam:       true,
			mockSave:           true,
			expectedError:      "",
		},
		{
			name:               "work type doesn't exist",
			workType:           "nonexistentType",
			setFromParamsError: nil,
			saveError:          nil,
			mockSetParam:       false,
			mockSave:           false,
			expectedError:      fmt.Sprintf("unknown work type %s", "nonexistentType"),
		},
		{
			name:               paramError,
			workType:           testType,
			setFromParamsError: errors.New(paramError),
			saveError:          nil,
			mockSetParam:       true,
			mockSave:           false,
			expectedError:      paramError,
		},
		{
			name:               saveError,
			workType:           testType,
			setFromParamsError: nil,
			saveError:          errors.New(saveError),
			mockSetParam:       true,
			mockSave:           true,
			expectedError:      saveError,
		},
	}

	checkError := func(err error, expectedError string, t *testing.T) {
		if expectedError == "" && err != nil {
			t.Errorf("Expected no error, got: %v", err)
		} else if expectedError != "" && (err == nil || err.Error() != expectedError) {
			t.Errorf("Expected error: %s, got: %v", expectedError, err)
		}
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mockSetParam {
				mockWorkUnit.EXPECT().SetFromParams(gomock.Any()).Return(tc.setFromParamsError).Times(1)
			}
			if tc.mockSave {
				mockWorkUnit.EXPECT().Save().Return(tc.saveError).Times(1)
			}

			_, err := w.AllocateUnit(tc.workType, map[string]string{"param": "value"})
			checkError(err, tc.expectedError, t)
		})
	}
}

func TestRegisterWithControlService(t *testing.T) {
	ctrl, _, w := testSetup(t)
	mockServer := mock_workceptor.NewMockServerForWorkceptor(ctrl)

	testCases := []struct {
		name          string
		hasError      bool
		expectedCalls func()
	}{
		{
			name:     "normal case 1",
			hasError: false,
			expectedCalls: func() {
				mockServer.EXPECT().AddControlFunc(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:     "error registering work",
			hasError: true,
			expectedCalls: func() {
				mockServer.EXPECT().AddControlFunc(gomock.Any(), gomock.Any()).Return(errors.New("terminated"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.expectedCalls()
			err := w.RegisterWithControlService(mockServer)
			if tc.hasError && err.Error() != "could not add work control function: terminated" {
				t.Error(err)
			}

			if !tc.hasError && err != nil {
				t.Error(err)
			}
		})
	}
}

func TestRegisterWorker(t *testing.T) {
	_, mockNetceptor, w := testSetup(t)

	testCases := []struct {
		name          string
		typeName      string
		hasError      bool
		errorMsg      string
		expectedCalls func()
	}{
		{
			name:     "already registered",
			typeName: "remote",
			hasError: true,
			errorMsg: "work type remote already registered",
			expectedCalls: func() {
				// For testing purposes
			},
		},
		{
			name:     "normal with active unit",
			typeName: "test",
			hasError: false,
			expectedCalls: func() {
				mockNetceptor.EXPECT().AddWorkCommand(gomock.Any(), gomock.Any())
				w.AllocateUnit("remote", map[string]string{})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.expectedCalls()
			err := w.RegisterWorker(tc.typeName, workceptor.NewRemoteWorker, false)
			if tc.hasError && err.Error() != tc.errorMsg {
				t.Error(err)
			}

			if !tc.hasError && err != nil {
				t.Error(err)
			}
		})
	}
}

func TestShouldVerifySignature(t *testing.T) {
	_, _, w := testSetup(t)

	testCases := []struct {
		name     string
		workType string
	}{
		{
			name:     "return with remote true",
			workType: "remote",
		},
		{
			name:     "return with false",
			workType: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shouldVerifySignature := w.ShouldVerifySignature(tc.workType, true)
			t.Log(shouldVerifySignature)
			if tc.workType == "remote" && !shouldVerifySignature {
				t.Errorf("expected: true, received: %t", shouldVerifySignature)
			}

			if tc.workType == "" && shouldVerifySignature {
				t.Errorf("expected: false, received: %t", shouldVerifySignature)
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	_, _, w := testSetup(t)

	_, public, err := utils.GenerateRSAPair()
	if err != nil {
		t.Error(err)
	}

	testCases := []struct {
		name         string
		signature    string
		verifyingKey string
		errorMsg     string
	}{
		{
			name:      "signature is empty error",
			signature: "",
			errorMsg:  "could not verify signature: signature is empty",
		},
		{
			name:         "verifying key not specified error",
			signature:    "sig",
			verifyingKey: "",
			errorMsg:     "could not verify signature: verifying key not specified",
		},
		{
			name:         "no such key file error",
			signature:    "sig",
			verifyingKey: "/tmp/nowhere.pub",
			errorMsg:     "could not load verifying key file: open /tmp/nowhere.pub: no such file or directory",
		},
		{
			name:         "token invalid number of segments error",
			signature:    "sig",
			verifyingKey: public,
			errorMsg:     "could not verify signature: token contains an invalid number of segments",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w.VerifyingKey = tc.verifyingKey
			err := w.VerifySignature(tc.signature)
			if tc.errorMsg != err.Error() {
				t.Errorf("expected: %s, received: %s", tc.errorMsg, err)
			}
		})
	}
}

func TestAllocateRemoteUnit(t *testing.T) {
	_, mockNetceptor, w := testSetup(t)

	testCases := []struct {
		name          string
		tlsClient     string
		ttl           string
		signWork      bool
		params        map[string]string
		errorMsg      string
		expectedCalls func()
	}{
		{
			name:      "get client tls config error",
			tlsClient: "something",
			errorMsg:  "terminated",
			expectedCalls: func() {
				mockNetceptor.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&tls.Config{}, errors.New("terminated"))
			},
		},
		{
			name:      "sending secrets over non tls connection error",
			tlsClient: "",
			params:    map[string]string{"secret_": "secret"},
			errorMsg:  "cannot send secrets over a non-TLS connection",
			expectedCalls: func() {
				// For testing purposes
			},
		},
		{
			name:      "invalid duration error",
			tlsClient: "",
			ttl:       "ttl",
			errorMsg:  "time: invalid duration \"ttl\"",
			expectedCalls: func() {
				// For testing purposes
			},
		},
		{
			name:      "normal case",
			tlsClient: "",
			ttl:       "1.5h",
			errorMsg:  "",
			signWork:  true,
			expectedCalls: func() {
				// For testing purposes
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.expectedCalls()
			_, err := w.AllocateRemoteUnit("", "", tc.tlsClient, tc.ttl, tc.signWork, tc.params)

			if tc.errorMsg != "" && tc.errorMsg != err.Error() && err != nil {
				t.Errorf("expected: %s, received: %s", tc.errorMsg, err)
			}

			if tc.errorMsg == "" && err != nil {
				t.Error(err)
			}
		})
	}
}

func TestUnitStatus(t *testing.T) {
	_, _, w := testSetup(t)
	activeUnitsIDs := w.ListKnownUnitIDs()

	_, err := w.UnitStatus(activeUnitsIDs[0])
	if err != nil {
		t.Error(err)
	}
}

func TestCancelUnit(t *testing.T) {
	_, _, w := testSetup(t)
	activeUnitsIDs := w.ListKnownUnitIDs()

	err := w.CancelUnit(activeUnitsIDs[0])
	if err != nil {
		t.Error(err)
	}
}

func TestReleaseUnit(t *testing.T) {
	_, _, w := testSetup(t)
	activeUnitsIDs := w.ListKnownUnitIDs()

	err := w.ReleaseUnit(activeUnitsIDs[0], true)
	if err != nil {
		t.Error(err)
	}
}

func TestListKnownUnitIDs(t *testing.T) {
	t.Parallel()
	_, _, w := testSetup(t)
	testCases := []struct {
		name     string
		workType string
	}{
		{
			name: "parallel test 1",
		},
		{
			name: "parallel test 2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w.ListKnownUnitIDs()
		})
	}
}
