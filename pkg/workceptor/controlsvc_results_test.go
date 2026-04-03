//go:build !no_workceptor
// +build !no_workceptor

package workceptor_test

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/utils/mock_utils"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"go.uber.org/mock/gomock"
)

// TestControlFuncResultsDoesNotCloseConnection verifies that the "results"
// subcommand does not call cfo.Close(). Closing the connection prematurely
// causes RunControlSession's read loop to get "use of closed network
// connection" instead of a clean EOF, producing spurious WARNING logs.
func TestControlFuncResultsDoesNotCloseConnection(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	tmpDir := t.TempDir()
	stdoutData := []byte("hello world")

	// Set up mock NetceptorForWorkceptor with expected function calls.
	mockNC := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
	mockNC.EXPECT().NodeID().Return("testnode").AnyTimes()
	mockNC.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()
	mockNC.EXPECT().AddWorkCommand("testwork", false).Return(nil)

	// Create Workceptor with the mocked netceptor.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w, err := workceptor.New(ctx, mockNC, tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Register a worker type with a factory that returns a MockWorkUnit.
	var mockUnit *mock_workceptor.MockWorkUnit
	factory := func(_ workceptor.BaseWorkUnitForWorkUnit, _ *workceptor.Workceptor, _ string, _ string) workceptor.WorkUnit {
		mockUnit = mock_workceptor.NewMockWorkUnit(ctrl)
		mockUnit.EXPECT().SetFromParams(gomock.Any()).Return(nil)
		mockUnit.EXPECT().Save().Return(nil)
		mockUnit.EXPECT().Status().Return(&workceptor.StatusFileData{
			State:      workceptor.WorkStateSucceeded,
			StdoutSize: int64(len(stdoutData)),
			WorkType:   "testwork",
		}).AnyTimes()

		return mockUnit
	}
	err = w.RegisterWorker("testwork", factory, false)
	if err != nil {
		t.Fatal(err)
	}

	// Allocate a unit so it exists in activeUnits.
	_, err = w.AllocateUnit("testwork", "testunit", map[string]string{})
	if err != nil {
		t.Fatal(err)
	}

	// Write stdout data to the unit directory created by AllocateUnit.
	unitDir := path.Join(tmpDir, "testnode", "testunit")
	if err := os.WriteFile(path.Join(unitDir, "stdout"), stdoutData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Capture the ControlCommandType via RegisterWithControlService.
	var cmdType controlsvc.ControlCommandType
	mockServer := mock_workceptor.NewMockServerForWorkceptor(ctrl)
	mockServer.EXPECT().AddControlFunc("work", gomock.Any()).DoAndReturn(
		func(_ string, ct controlsvc.ControlCommandType) error {
			cmdType = ct

			return nil
		},
	)
	err = w.RegisterWithControlService(mockServer)
	if err != nil {
		t.Fatal(err)
	}

	// Create a "results" command to simulate getting work results
	// using receptorctl.
	cmd, err := cmdType.InitFromJSON(map[string]interface{}{
		"command":    "work",
		"subcommand": "results",
		"unitid":     "testunit",
		"startpos":   int64(0),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set up mock ControlFuncOperations. This mocks the cfo parameter
	// in workceptor/controlsvc.go::ControlFunc that provides the
	// Close() func which is intentionally not expected to be called.
	mockCfo := mock_controlsvc.NewMockControlFuncOperations(ctrl)
	mockAddr := mock_utils.NewMockNetAddr(ctrl)
	mockAddr.EXPECT().Network().Return("unix")
	mockCfo.EXPECT().RemoteAddr().Return(mockAddr)
	mockCfo.EXPECT().WriteToConn(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ string, ch chan []byte) error {
			// Drain the channel so the GetResults goroutine can complete.
			for b := range ch {
				_ = b
			}

			return nil
		},
	)
	// Intentionally do NOT set up mockCfo.EXPECT().Close().

	// Mock NetceptorForControlCommand (nc parameter passed to ControlFunc).
	mockNCCmd := mock_controlsvc.NewMockNetceptorForControlCommand(ctrl)

	// Execute the command.
	result, err := cmd.ControlFunc(ctx, mockNCCmd, mockCfo)
	if err != nil {
		t.Fatalf("ControlFunc returned unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("ControlFunc returned unexpected result: %v", result)
	}
}
