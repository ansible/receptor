package workceptor_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	gomock "go.uber.org/mock/gomock"
)

func TestAllocateUnit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWorkUnit := mock_workceptor.NewMockWorkUnit(ctrl)
	ctx := context.Background()
	mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)

	logger := logger.NewReceptorLogger("")
	mockNetceptor.EXPECT().GetLogger().AnyTimes().Return(logger)

	workFunc := func(w *workceptor.Workceptor, unitID string, workType string) workceptor.WorkUnit {
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
