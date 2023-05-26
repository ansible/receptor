package workceptor_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"github.com/golang/mock/gomock"
)

func TestAllocateUnit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWorkUnit := mock_workceptor.NewMockWorkUnit(ctrl)
	mockWorkUnit.EXPECT().SetFromParams(gomock.Any()).Return(nil).Times(1)
	mockWorkUnit.EXPECT().Save().Return(nil).Times(1)
	ctx := context.Background()
	mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)

	// attach logger to the mock netceptor and return any number of times
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

	mockNetceptor.EXPECT().AddWorkCommand(gomock.Any(), gomock.Any()).Return(nil)
	w.RegisterWorker("testType", workFunc, false)
	// Test a normal case
	_, err = w.AllocateUnit("testType", map[string]string{"param": "value"})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Test with a work type that doesn't exist
	_, err = w.AllocateUnit("nonexistentType", map[string]string{"param": "value"})
	if err == nil || err.Error() != fmt.Errorf("unknown work type %s", "nonexistentType").Error() {
		t.Errorf("Expected 'unknown work type %s', got: %v", "nonexistentType", err)
	}

	// Test with a SetFromParams that returns an error
	mockWorkUnit.EXPECT().SetFromParams(gomock.Any()).Return(errors.New("SetFromParams error"))
	_, err = w.AllocateUnit("testType", map[string]string{"param": "value"})
	if err == nil || err.Error() != "SetFromParams error" {
		t.Errorf("Expected 'SetFromParams error', got: %v", err)
	}

	// Test with a Save that returns an error
	mockWorkUnit.EXPECT().SetFromParams(gomock.Any()).Return(nil)
	mockWorkUnit.EXPECT().Save().Return(errors.New("Save error"))
	_, err = w.AllocateUnit("testType", map[string]string{"param": "value"})
	if err == nil || err.Error() != "Save error" {
		t.Errorf("Expected 'Save error', got: %v", err)
	}
}
