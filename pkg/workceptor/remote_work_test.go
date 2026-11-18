package workceptor_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/netceptor/mock_netceptor"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	testutils "github.com/ansible/receptor/tests/utils"
	"github.com/prep/socketpair"
	"go.uber.org/mock/gomock"
)

// createRemoteWorkNetworkSetup creates a mock network Conn for testing remote work operations.
// It takes a list of messages to be sent to the mock Conn and sets up the mock netceptor and base work unit expectations.
func createRemoteWorkNetworkSetup(t *testing.T, ctrl *gomock.Controller, ctx context.Context, messages []string, mockNetceptor *mock_workceptor.MockNetceptorForWorkceptor, mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, tmpDir string, remoteExtraData *workceptor.RemoteExtraData, anytimes bool) {
	t.Helper()

	// Create a mock Conn using the mock interfaces
	mockPacketConner := mock_netceptor.NewMockPacketConner(ctrl)
	mockQuicConnection := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	mockQuicStream := mock_netceptor.NewMockQuicStreamForConn(ctrl)

	messageIndex := 0
	// ReadStringContext's internal goroutine can outlive the call that started it, so a
	// stale reader may still invoke Read after the caller has reconnected and started a
	// new Read on the same mock stream. Protect the shared cursor against that overlap.
	var messageIndexLock sync.Mutex
	// Set up reads to return different messages based on the provided list
	readExpectation := mockQuicStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(b []byte) (int, error) {
		messageIndexLock.Lock()
		defer messageIndexLock.Unlock()
		if messageIndex < len(messages) {
			msg := messages[messageIndex]
			messageIndex++
			copy(b, msg)

			return len(msg), nil
		}

		return 0, ctx.Err()
	})
	if anytimes {
		readExpectation.AnyTimes()
	} else {
		readExpectation.Times(len(messages))
	}
	mockQuicStream.EXPECT().Write(gomock.Any()).Return(0, nil).AnyTimes()
	mockQuicStream.EXPECT().Close().Return(nil).AnyTimes()
	mockQuicStream.EXPECT().SetReadDeadline(gomock.Any()).Return(nil).AnyTimes()
	mockQuicStream.EXPECT().SetWriteDeadline(gomock.Any()).Return(nil).AnyTimes()
	mockQuicStream.EXPECT().SetDeadline(gomock.Any()).Return(nil).AnyTimes()

	var cancelFunc context.CancelFunc = func() {}
	mockPacketConner.EXPECT().Cancel().Return(&cancelFunc).AnyTimes()
	mockPacketConner.EXPECT().Close().Return(nil).AnyTimes()
	mockPacketConner.EXPECT().LocalService().Return("test-service").AnyTimes()
	mockPacketConner.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()

	mockQuicConnection.EXPECT().CloseWithError(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockQuicConnection.EXPECT().RemoteAddr().Return(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}).AnyTimes()
	mockQuicConnection.EXPECT().LocalAddr().Return(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}).AnyTimes()

	mockConn := netceptor.NewConn(
		netceptor.New(ctx, "test-node"), // s: *Netceptor
		mockPacketConner,                // pc: PacketConner
		mockQuicConnection,              // qc: QuicConnectionForConn
		mockQuicStream,                  // qs: QuicStreamForConn
		make(chan struct{}),             // doneChan
		&sync.Once{},                    // doneOnce
		ctx,                             // ctx
	)

	// Set up mock netceptor expectations
	mockNetceptor.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&tls.Config{}, nil).AnyTimes()
	mockNetceptor.EXPECT().DialContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockConn, nil).AnyTimes()

	// Set up common mock base work unit expectations
	mockBaseWorkUnit.EXPECT().Load().Return(nil).AnyTimes()
	mockBaseWorkUnit.EXPECT().StdoutFileName().Return(filepath.Join(tmpDir, "stdout")).AnyTimes()
	mockBaseWorkUnit.EXPECT().UnitDir().Return(tmpDir).AnyTimes()
	mockBaseWorkUnit.EXPECT().UpdateFullStatus(gomock.Any()).Do(func(updateFunc interface{}) {
		updateFunc.(func(*workceptor.StatusFileData))(&workceptor.StatusFileData{
			ExtraData: remoteExtraData,
		})
	}).AnyTimes()
	mockBaseWorkUnit.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockBaseWorkUnit.EXPECT().LastUpdateError().Return(nil).AnyTimes()

	// Create temporary directory and stdin file
	_ = os.MkdirAll(tmpDir, 0o755)
	stdinFile, err := os.Create(filepath.Join(tmpDir, "stdin"))
	if err != nil {
		t.Errorf("Error creating temporary file: %v", err)
	} else {
		stdinFile.Close()
		t.Cleanup(func() {
			os.Remove(filepath.Join(tmpDir, "stdin"))
		})
	}
}

// createRemoteWorkTestSetup creates mocks for testing remote work units.
// Note: ctrl.Finish() is automatically called via t.Cleanup() when using gomock.NewController(t).
func createRemoteWorkTestSetup(t *testing.T, ctx context.Context) (workceptor.WorkUnit, *mock_workceptor.MockBaseWorkUnitForWorkUnit, *mock_workceptor.MockNetceptorForWorkceptor, *workceptor.Workceptor, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mockBaseWorkUnit := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
	mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
	mockNetceptor.EXPECT().NodeID().Return("NodeID")
	mockNetceptor.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()

	w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
	if err != nil {
		t.Errorf("Error while creating Workceptor: %v", err)
	}
	t.Cleanup(w.Cancel)

	mockBaseWorkUnit.EXPECT().Init(w, "", "", workceptor.FileSystem{})
	mockBaseWorkUnit.EXPECT().SetStatusExtraData(gomock.Any())
	workUnit := workceptor.NewRemoteWorker(mockBaseWorkUnit, w, "", "")

	return workUnit, mockBaseWorkUnit, mockNetceptor, w, ctrl
}

// expectRemoteUnitStatusSources stubs the read-only status accessors that
// remote work unit monitoring calls on its base unit.
func expectRemoteUnitStatusSources(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, w *workceptor.Workceptor, red *workceptor.RemoteExtraData, status workceptor.StatusFileData) {
	// gomock evaluates Return once: every GetStatusLock() call returns this same
	// mutex, which remoteUnit.UnredactedStatus relies on (it takes RLock and
	// RUnlock via separate calls).
	statusLock := &sync.RWMutex{}
	mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).AnyTimes()
	mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().DoAndReturn(func() *workceptor.StatusFileData {
		s := status

		return &s
	}).AnyTimes()
	mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{ExtraData: red}).AnyTimes()
	mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
	mockBaseWorkUnit.EXPECT().ID().Return("test-id").AnyTimes()
}

func TestRemoteWorkUnredactedStatus(t *testing.T) {
	ctx := context.Background()
	wu, mockBaseWorkUnit, _, _, _ := createRemoteWorkTestSetup(t, ctx) //nolint:dogsled
	restartTestCases := []struct {
		name string
	}{
		{name: "test1"},
		{name: "test2"},
	}

	statusLock := &sync.RWMutex{}
	for _, testCase := range restartTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).Times(2)
			mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().DoAndReturn(func() *workceptor.StatusFileData {
				return &workceptor.StatusFileData{}
			})
			mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
				ExtraData: &workceptor.RemoteExtraData{},
			})
			wu.UnredactedStatus()
		})
	}
}

func TestRemoteWorkSetFromParams(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	wu, mockBaseWorkUnit, _, _, _ := createRemoteWorkTestSetup(t, ctx) //nolint:dogsled

	params := map[string]string{
		"param1": "value1",
		"param2": "value2",
	}

	remoteExtraData := &workceptor.RemoteExtraData{
		RemoteParams: make(map[string]string),
	}

	mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
		ExtraData: remoteExtraData,
	}).Times(2) // Called multiple times during SetFromParams

	err := wu.SetFromParams(params)
	if err != nil {
		t.Errorf("SetFromParams failed: %v", err)
	}
}

func TestRemoteWorkStatusRedaction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		remoteParams   map[string]string
		expectedSecret bool
		expectedPublic bool
	}{
		{
			name: "secrets are redacted",
			remoteParams: map[string]string{
				"secret_password": "hidden",
				"public_param":    "visible",
			},
			expectedSecret: false,
			expectedPublic: true,
		},
		{
			name: "no secrets to redact",
			remoteParams: map[string]string{
				"param1": "value1",
				"param2": "value2",
			},
			expectedSecret: false,
			expectedPublic: true,
		},
		{
			name: "multiple secrets redacted",
			remoteParams: map[string]string{
				"secret_key":     "hidden1",
				"SECRET_TOKEN":   "hidden2",
				"public_setting": "visible",
			},
			expectedSecret: false,
			expectedPublic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			wu, mockBaseWorkUnit, _, _, _ := createRemoteWorkTestSetup(t, ctx)

			statusLock := &sync.RWMutex{}
			mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).Times(2)
			mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().DoAndReturn(func() *workceptor.StatusFileData {
				return &workceptor.StatusFileData{}
			})
			mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
				ExtraData: &workceptor.RemoteExtraData{
					RemoteParams: tt.remoteParams,
				},
			})

			status := wu.Status()
			red := status.ExtraData.(*workceptor.RemoteExtraData)

			// Check for secrets
			hasSecret := false
			for k := range red.RemoteParams {
				if strings.HasPrefix(strings.ToLower(k), "secret_") {
					hasSecret = true

					break
				}
			}

			if hasSecret != tt.expectedSecret {
				t.Errorf("Expected secrets present: %v, got: %v", tt.expectedSecret, hasSecret)
			}

			// Check for public params
			hasPublic := false
			for k := range red.RemoteParams {
				if !strings.HasPrefix(strings.ToLower(k), "secret_") {
					hasPublic = true

					break
				}
			}

			if hasPublic != tt.expectedPublic {
				t.Errorf("Expected public params present: %v, got: %v", tt.expectedPublic, hasPublic)
			}
		})
	}
}

func TestRemoteWorkLifecycleOperations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		operation     string
		remoteStarted bool
		expectError   bool
		errorContains string
	}{
		{
			name:      "start_not_started",
			operation: "start",
		},
		{
			name:          "start_already_started",
			operation:     "start",
			remoteStarted: true,
			expectError:   true,
			errorContains: "unit was already started",
		},
		{
			name:          "restart_not_started",
			operation:     "restart",
			expectError:   true,
			errorContains: "remote work had not previously started",
		},
		{
			name:          "restart_already_started",
			operation:     "restart",
			remoteStarted: true,
			expectError:   false,
		},
		{
			name:          "restart_local_released",
			operation:     "restart",
			expectError:   true,
			errorContains: "remote work had not previously started",
		},
		{
			name:          "restart_local_cancelled",
			operation:     "restart",
			remoteStarted: true,
			expectError:   false,
		},
		{
			name:          "restart_unknown_work_unit",
			operation:     "restart",
			remoteStarted: true,
			expectError:   false, // No error expected because the error is handled via the status file
		},
		{
			name:      "cancel_not_started",
			operation: "cancel",
		},
		{
			name:          "cancel_already_started",
			operation:     "cancel",
			remoteStarted: true,
		},
		{
			name:      "release_not_started",
			operation: "release",
		},
		{
			name:          "release_already_started",
			operation:     "release",
			remoteStarted: true,
		},
		{
			name:          "force_release_already_started",
			operation:     "release",
			remoteStarted: true,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Each test creates its own cancellable context to stop goroutines
			// Each test creates its own gomock Controller instance

			t.Parallel()
			contextWithCancel, cancel := context.WithCancel(context.Background())
			wu, mockBaseWorkUnit, mockNetceptor, w, ctrl := createRemoteWorkTestSetup(t, contextWithCancel)

			t.Cleanup(func() {
				cancel() // Signal goroutines to stop
				// Wait for goroutines to finish
				time.Sleep(200 * time.Millisecond)
			})

			remoteExtraData := &workceptor.RemoteExtraData{
				RemoteStarted: tt.remoteStarted,
				RemoteNode:    "execution",
				RemoteParams:  make(map[string]string),
			}

			statusLock := &sync.RWMutex{}
			mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().DoAndReturn(func() *workceptor.StatusFileData {
				return &workceptor.StatusFileData{}
			}).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
				ExtraData: remoteExtraData,
			}).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()

			if tt.operation == "cancel" || tt.operation == "release" {
				mockBaseWorkUnit.EXPECT().UpdateFullStatus(gomock.Any()).Do(func(updateFunc interface{}) {
					updateFunc.(func(*workceptor.StatusFileData))(&workceptor.StatusFileData{
						ExtraData: remoteExtraData,
					})
				})
			}

			// Use test case name for temp directory
			tmpDir := filepath.Join("/tmp", tt.name)

			var err error
			switch tt.name {
			case "start_not_started":
				mockBaseWorkUnit.EXPECT().ID().Return("test-id").Times(1)
				messages := []string{
					"execution\n", // Hello message with remote node ID
					"Work unit created with ID execution. Send stdin data and EOF.\n",
					"OK\n", // Acknowledgment after stdin sent
					"{\"State\": 1, \"Detail\": \"Running\", \"StdoutSize\": 0}\n", // Status updates for monitoring
				}
				anyTimes := true // Needed because this will monitor the remote work unit in a loop
				createRemoteWorkNetworkSetup(t, ctrl, contextWithCancel, messages, mockNetceptor, mockBaseWorkUnit, tmpDir, remoteExtraData, anyTimes)
				err = wu.Start()
			case "start_already_started":
				err = wu.Start()
			case "restart_not_started":
				err = wu.Restart()
			case "restart_already_started":
				messages := []string{
					"execution\n", // Hello message with remote node ID
					"{\"State\": 1, \"Detail\": \"Running\", \"StdoutSize\": 0}\n", // Response to status command
				}
				anyTimes := true // Needed because this will monitor the remote work unit in a loop
				createRemoteWorkNetworkSetup(t, ctrl, contextWithCancel, messages, mockNetceptor, mockBaseWorkUnit, tmpDir, remoteExtraData, anyTimes)
				err = wu.Restart()
			case "restart_local_released":
				remoteExtraData.LocalReleased = true
				err = wu.Restart()
			case "restart_local_cancelled":
				// This test verifies that when LocalCancelled=true but LocalReleased=false,
				// the work unit directory is NOT deleted (Release is not called).
				// This was a bug fix: previously forRelease was hardcoded to true,
				// causing premature deletion of cancelled-but-not-released work units.
				remoteExtraData.LocalCancelled = true
				remoteExtraData.LocalReleased = false
				messages := []string{
					"execution\n", // Hello message with remote node ID
					"{\"State\": 4, \"Detail\": \"Cancelled\", \"StdoutSize\": 0}\n", // Response to cancel command
				}
				anyTimes := true
				createRemoteWorkNetworkSetup(t, ctrl, contextWithCancel, messages, mockNetceptor, mockBaseWorkUnit, tmpDir, remoteExtraData, anyTimes)
				// Note: We do NOT expect mockBaseWorkUnit.Release() to be called.
				// If Release() were called, gomock would fail with an unexpected call error.
				err = wu.Restart()
			case "restart_unknown_work_unit":
				messages := []string{
					"execution\n", // Hello message with remote node ID
					"ERROR: unknown work unit\n",
				}
				anyTimes := true // Needed because this will monitor the remote work unit in a loop
				createRemoteWorkNetworkSetup(t, ctrl, contextWithCancel, messages, mockNetceptor, mockBaseWorkUnit, tmpDir, remoteExtraData, anyTimes)
				err = wu.Restart()
			case "cancel_not_started":
				mockBaseWorkUnit.EXPECT().UpdateBasicStatus(workceptor.WorkStateFailed, "Locally Cancelled", int64(0))
				err = wu.Cancel()
			case "cancel_already_started":
				messages := []string{
					"execution\n", // Hello message with remote node ID
					"{\"State\": 4, \"Detail\": \"Cancelled\", \"StdoutSize\": 0}\n", // Acknowledgment to release command
				}
				anyTimes := true // Needed because this will monitor the remote work unit in a loop
				createRemoteWorkNetworkSetup(t, ctrl, contextWithCancel, messages, mockNetceptor, mockBaseWorkUnit, tmpDir, remoteExtraData, anyTimes)
				err = wu.Cancel()
			case "release_not_started":
				mockBaseWorkUnit.EXPECT().Release(true).Return(nil)
				err = wu.Release(false)
			case "release_already_started":
				messages := []string{
					"execution\n", // Hello message with remote node ID
					"{\"State\": 4, \"Detail\": \"Cancelled\", \"StdoutSize\": 0}\n", // Acknowledgment to release command
				}
				createRemoteWorkNetworkSetup(t, ctrl, contextWithCancel, messages, mockNetceptor, mockBaseWorkUnit, tmpDir, remoteExtraData, false)
				mockBaseWorkUnit.EXPECT().Release(false).Return(nil)
				err = wu.Release(false)
			case "force_release_already_started":
				messages := []string{
					"execution\n", // Hello message with remote node ID
					"{\"State\": 4, \"Detail\": \"Cancelled\", \"StdoutSize\": 0}\n", // Acknowledgment to release command
				}
				anyTimes := true // Needed because this will monitor the remote work unit in a loop
				createRemoteWorkNetworkSetup(t, ctrl, contextWithCancel, messages, mockNetceptor, mockBaseWorkUnit, tmpDir, remoteExtraData, anyTimes)
				mockBaseWorkUnit.EXPECT().Release(true).Return(nil)
				err = wu.Release(true)
			default:
				t.Errorf("Unknown test case: %s", tt.name)
			}

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for %s operation", tt.operation)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s operation: %v", tt.operation, err)
				}
			}
		})
	}
}

func TestRemoteWorkStartWithExpiration(t *testing.T) {
	t.Parallel()
	contextWithCancel, cancel := context.WithCancel(context.Background())
	wu, mockBaseWorkUnit, mockNetceptor, w, ctrl := createRemoteWorkTestSetup(t, contextWithCancel)

	t.Cleanup(func() {
		cancel()
		time.Sleep(200 * time.Millisecond)
	})

	remoteExtraData := &workceptor.RemoteExtraData{
		RemoteNode:   "execution",
		RemoteParams: make(map[string]string),
		Expiration:   time.Now().Add(time.Hour),
	}

	statusLock := &sync.RWMutex{}
	mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).AnyTimes()
	mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().DoAndReturn(func() *workceptor.StatusFileData {
		return &workceptor.StatusFileData{}
	}).AnyTimes()
	mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
		ExtraData: remoteExtraData,
	}).AnyTimes()
	mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
	mockBaseWorkUnit.EXPECT().ID().Return("test-id").Times(1)

	tmpDir := filepath.Join("/tmp", "start_with_expiration")
	messages := []string{
		"execution\n",
		"Work unit created with ID execution. Send stdin data and EOF.\n",
		"OK\n",
		"{\"State\": 1, \"Detail\": \"Running\", \"StdoutSize\": 0}\n",
	}
	createRemoteWorkNetworkSetup(t, ctrl, contextWithCancel, messages, mockNetceptor, mockBaseWorkUnit, tmpDir, remoteExtraData, true)

	if err := wu.Start(); err != nil {
		t.Errorf("Unexpected error starting work unit with expiration: %v", err)
	}
}

func TestRemoteWorkConnectToRemoteEnhanced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		tlsClientName string
		remoteNode    string
		tlsError      error
		dialError     error
		expectError   bool
		errorContains string
	}{
		{
			name:          "missing extra data",
			expectError:   true,
			errorContains: "remote ExtraData missing",
		},
		{
			name:          "TLS config error",
			tlsClientName: "test-client",
			remoteNode:    "test-node",
			tlsError:      fmt.Errorf("TLS configuration failed"),
			expectError:   true,
			errorContains: "TLS configuration failed",
		},
		{
			name:          "dial context error",
			tlsClientName: "test-client",
			remoteNode:    "test-node",
			dialError:     fmt.Errorf("connection refused"),
			expectError:   true,
			errorContains: "connection refused",
		},
		{
			name:          "dial context timeout",
			tlsClientName: "test-client",
			remoteNode:    "test-node",
			dialError:     fmt.Errorf("context deadline exceeded"),
			expectError:   true,
			errorContains: "context deadline exceeded",
		},
		{
			name:          "TLS config nil client name",
			tlsClientName: "invalid-client",
			remoteNode:    "test-node",
			tlsError:      fmt.Errorf("client certificate not found"),
			expectError:   true,
			errorContains: "client certificate not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			wu, mockBaseWorkUnit, mockNetceptor, w, _ := createRemoteWorkTestSetup(t, ctx)

			// Set up remote extra data
			remoteExtraData := &workceptor.RemoteExtraData{
				RemoteNode:     tt.remoteNode,
				TLSClient:      tt.tlsClientName,
				RemoteWorkType: "test-work",
				RemoteParams:   make(map[string]string),
			}

			// Configure basic mock expectations
			statusLock := &sync.RWMutex{}
			mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().DoAndReturn(func() *workceptor.StatusFileData {
				return &workceptor.StatusFileData{}
			}).AnyTimes()

			// Handle missing extra data case
			if tt.name == "missing extra data" {
				mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
					ExtraData: "invalid", // Wrong type
				}).AnyTimes()
			} else {
				mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
					ExtraData: remoteExtraData,
				}).AnyTimes()
			}

			mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
			// Configure netceptor mock expectations based on test case
			if tt.tlsError != nil {
				mockNetceptor.EXPECT().GetClientTLSConfig(tt.tlsClientName, tt.remoteNode, gomock.Any()).Return(nil, tt.tlsError)
			} else if tt.tlsClientName != "" {
				mockNetceptor.EXPECT().GetClientTLSConfig(tt.tlsClientName, tt.remoteNode, gomock.Any()).Return(&tls.Config{}, nil)
			}

			if tt.dialError != nil {
				mockNetceptor.EXPECT().DialContext(gomock.Any(), tt.remoteNode, "control", gomock.Any()).Return(nil, tt.dialError)
			}

			// Test ConnectToRemote
			if rw, ok := wu.(interface {
				ConnectToRemote(context.Context) (net.Conn, *bufio.Reader, error)
			}); ok {
				ctx := context.Background()
				conn, reader, err := rw.ConnectToRemote(ctx)

				if tt.expectError {
					if err == nil {
						t.Errorf("Expected error but got none")
					} else if !strings.Contains(err.Error(), tt.errorContains) {
						t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
					}
					if conn != nil {
						t.Error("Expected connection to be nil on error")
					}
					if reader != nil {
						t.Error("Expected reader to be nil on error")
					}
				}
				if !tt.expectError && err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			} else {
				t.Error("WorkUnit doesn't implement ConnectToRemote method")
			}
		})
	}
}

func TestRemoteWorkGetConnectionCryptoErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		dialError         error
		extraData         interface{}
		expectStateFailed bool
		expectContains    []string
	}{
		{
			name:      "CRYPTO_BUFFER_EXCEEDED sets helpful message with KCS reference",
			dialError: fmt.Errorf("CRYPTO_BUFFER_EXCEEDED (local): received invalid offset 17125 on crypto stream, maximum allowed 16384"),
			extraData: &workceptor.RemoteExtraData{
				RemoteNode: "execution", TLSClient: "test-client",
				RemoteWorkType: "test-work", RemoteParams: make(map[string]string), RemoteStarted: false,
			},
			expectStateFailed: true,
			expectContains:    []string{"KCS 7129200", "16KB", "QUIC crypto buffer exceeded", "CA bundle", "too large"},
		},
		{
			name:      "CRYPTO_ERROR sets TLS error message",
			dialError: fmt.Errorf("CRYPTO_ERROR: TLS handshake failed"),
			extraData: &workceptor.RemoteExtraData{
				RemoteNode: "execution", TLSClient: "test-client",
				RemoteWorkType: "test-work", RemoteParams: make(map[string]string), RemoteStarted: false,
			},
			expectStateFailed: true,
			expectContains:    []string{"TLS error connecting to remote service", "CRYPTO_ERROR"},
		},
		{
			name:              "nil ExtraData handles gracefully",
			dialError:         fmt.Errorf("CRYPTO_BUFFER_EXCEEDED: test"),
			extraData:         nil,
			expectStateFailed: false, // Defensive check prevents State update
			expectContains:    []string{"KCS 7129200"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			wu, mockBaseWorkUnit, mockNetceptor, w, _ := createRemoteWorkTestSetup(t, ctx)

			remoteExtraData := &workceptor.RemoteExtraData{
				RemoteNode:     "execution",
				TLSClient:      "test-client",
				RemoteWorkType: "test-work",
				RemoteParams:   make(map[string]string),
				RemoteStarted:  false,
			}

			statusLock := &sync.RWMutex{}
			var capturedDetail string
			var capturedState int

			// Set up mocks
			mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().DoAndReturn(func() *workceptor.StatusFileData {
				return &workceptor.StatusFileData{}
			}).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
				ExtraData: remoteExtraData,
			}).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()

			// Capture status update - use tt.extraData for different scenarios
			mockBaseWorkUnit.EXPECT().UpdateFullStatus(gomock.Any()).Do(func(updateFunc interface{}) {
				status := &workceptor.StatusFileData{
					ExtraData: tt.extraData,
				}
				updateFunc.(func(*workceptor.StatusFileData))(status)
				capturedDetail = status.Detail
				capturedState = status.State
			}).AnyTimes()

			// Mock TLS config and DialContext with test-specific error
			mockNetceptor.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&tls.Config{}, nil).AnyTimes()
			mockNetceptor.EXPECT().DialContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, tt.dialError).AnyTimes()

			// Call GetConnection
			conn, reader := wu.(interface {
				GetConnection(context.Context) (net.Conn, *bufio.Reader)
			}).GetConnection(ctx)

			// Verify connection failed
			if conn != nil || reader != nil {
				t.Error("Expected nil connection and reader")
			}

			// Verify status detail contains expected strings
			for _, expected := range tt.expectContains {
				if !strings.Contains(capturedDetail, expected) {
					t.Errorf("Expected status detail to contain '%s', got: %s", expected, capturedDetail)
				}
			}

			// Verify State
			if tt.expectStateFailed && capturedState != workceptor.WorkStateFailed {
				t.Errorf("Expected State=Failed, got: %d", capturedState)
			}
			if !tt.expectStateFailed && capturedState != 0 {
				t.Errorf("Expected State=0, got: %d", capturedState)
			}
		})
	}
}

func TestGetConnectionSurfacesRetryableErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		dialError      error
		extraData      interface{}
		expectContains string
	}{
		{
			name:      "retryable error writes Detail with remote node name",
			dialError: fmt.Errorf("connection refused"),
			extraData: &workceptor.RemoteExtraData{
				RemoteNode: "execution", TLSClient: "test-client",
				RemoteWorkType: "test-work", RemoteParams: make(map[string]string),
			},
			expectContains: "Connection to execution failed: connection refused",
		},
		{
			name:           "retryable error without RemoteExtraData uses fallback",
			dialError:      fmt.Errorf("dial timeout"),
			extraData:      nil,
			expectContains: "Connection to remote node failed: dial timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 1.5s allows two GetConnection iterations (1s retry delay) so the
			// idempotency check (status.Detail == newDetail → return) fires on
			// the second call with the same error.
			ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
			defer cancel()

			wu, mockBaseWorkUnit, mockNetceptor, w, _ := createRemoteWorkTestSetup(t, ctx)

			remoteExtraData := &workceptor.RemoteExtraData{
				RemoteNode:     "execution",
				TLSClient:      "test-client",
				RemoteWorkType: "test-work",
				RemoteParams:   make(map[string]string),
			}

			statusLock := &sync.RWMutex{}
			var capturedDetail string

			mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().DoAndReturn(func() *workceptor.StatusFileData {
				return &workceptor.StatusFileData{}
			}).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
				ExtraData: remoteExtraData,
			}).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
			mockBaseWorkUnit.EXPECT().UpdateFullStatus(gomock.Any()).Do(func(updateFunc interface{}) {
				status := &workceptor.StatusFileData{
					Detail:    capturedDetail,
					ExtraData: tt.extraData,
				}
				updateFunc.(func(*workceptor.StatusFileData))(status)
				capturedDetail = status.Detail
			}).AnyTimes()

			mockNetceptor.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&tls.Config{}, nil).AnyTimes()
			mockNetceptor.EXPECT().DialContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, tt.dialError).AnyTimes()

			conn, reader := wu.(interface {
				GetConnection(context.Context) (net.Conn, *bufio.Reader)
			}).GetConnection(ctx)

			if conn != nil || reader != nil {
				t.Error("Expected nil connection and reader")
			}
			if !strings.Contains(capturedDetail, tt.expectContains) {
				t.Errorf("Expected detail to contain %q, got: %s", tt.expectContains, capturedDetail)
			}
		})
	}
}

func TestGetConnectionDoesNotOverwriteTerminalDetail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state int
		want  string
	}{
		{name: "succeeded", state: workceptor.WorkStateSucceeded, want: "original detail"},
		{name: "failed", state: workceptor.WorkStateFailed, want: "original detail"},
		{name: "canceled", state: workceptor.WorkStateCanceled, want: "original detail"},
		{name: "running", state: workceptor.WorkStateRunning, want: "Connection to execution failed: connection refused"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			wu, mockBaseWorkUnit, mockNetceptor, w, _ := createRemoteWorkTestSetup(t, ctx)

			remoteExtraData := &workceptor.RemoteExtraData{
				RemoteNode:     "execution",
				TLSClient:      "test-client",
				RemoteWorkType: "test-work",
				RemoteParams:   make(map[string]string),
			}

			statusLock := &sync.RWMutex{}
			detail := "original detail"

			mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().DoAndReturn(func() *workceptor.StatusFileData {
				return &workceptor.StatusFileData{}
			}).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
				ExtraData: remoteExtraData,
			}).AnyTimes()
			mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
			mockBaseWorkUnit.EXPECT().UpdateFullStatus(gomock.Any()).Do(func(updateFunc interface{}) {
				status := &workceptor.StatusFileData{
					State:     tt.state,
					Detail:    detail,
					ExtraData: remoteExtraData,
				}
				updateFunc.(func(*workceptor.StatusFileData))(status)
				detail = status.Detail
			}).AnyTimes()

			mockNetceptor.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&tls.Config{}, nil).AnyTimes()
			mockNetceptor.EXPECT().DialContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, fmt.Errorf("connection refused")).AnyTimes()

			wu.(interface {
				GetConnection(context.Context) (net.Conn, *bufio.Reader)
			}).GetConnection(ctx)

			if detail != tt.want {
				t.Errorf("Expected detail %q, got %q", tt.want, detail)
			}
		})
	}
}

// statusDetailRecorder holds the unit Detail that the monitor goroutines update concurrently.
type statusDetailRecorder struct {
	mu     sync.Mutex
	detail string
}

func (r *statusDetailRecorder) apply(updateFunc interface{}, extraData *workceptor.RemoteExtraData) {
	r.mu.Lock()
	defer r.mu.Unlock()
	status := &workceptor.StatusFileData{Detail: r.detail, ExtraData: extraData}
	updateFunc.(func(*workceptor.StatusFileData))(status)
	r.detail = status.Detail
}

func (r *statusDetailRecorder) get() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.detail
}

func waitForDetail(r *statusDetailRecorder, want string) bool {
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if strings.Contains(r.get(), want) {
			return true
		}
	}

	return false
}

// newStatusStreamErrorConn returns a conn whose handshake succeeds and whose status
// exchange fails: writes fail when failWrite is set, otherwise reads fail.
func newStatusStreamErrorConn(ctx context.Context, ctrl *gomock.Controller, failWrite bool) *netceptor.Conn {
	mockPacketConner := mock_netceptor.NewMockPacketConner(ctrl)
	mockQuicConnection := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	mockQuicStream := mock_netceptor.NewMockQuicStreamForConn(ctrl)

	helloSent := false
	mockQuicStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(b []byte) (int, error) {
		if helloSent {
			return 0, fmt.Errorf("connection reset")
		}
		helloSent = true
		msg := "execution\n"
		copy(b, msg)

		return len(msg), nil
	}).AnyTimes()
	mockQuicStream.EXPECT().Write(gomock.Any()).DoAndReturn(func(b []byte) (int, error) {
		if failWrite {
			return 0, fmt.Errorf("broken pipe")
		}

		return len(b), nil
	}).AnyTimes()
	mockQuicStream.EXPECT().Close().Return(nil).AnyTimes()
	mockQuicStream.EXPECT().SetReadDeadline(gomock.Any()).Return(nil).AnyTimes()
	mockQuicStream.EXPECT().SetWriteDeadline(gomock.Any()).Return(nil).AnyTimes()
	mockQuicStream.EXPECT().SetDeadline(gomock.Any()).Return(nil).AnyTimes()

	var cancelFunc context.CancelFunc = func() {}
	mockPacketConner.EXPECT().Cancel().Return(&cancelFunc).AnyTimes()
	mockPacketConner.EXPECT().Close().Return(nil).AnyTimes()
	mockPacketConner.EXPECT().LocalService().Return("test-service").AnyTimes()
	mockPacketConner.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()

	mockQuicConnection.EXPECT().CloseWithError(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockQuicConnection.EXPECT().RemoteAddr().Return(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}).AnyTimes()
	mockQuicConnection.EXPECT().LocalAddr().Return(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}).AnyTimes()

	return netceptor.NewConn(
		netceptor.New(ctx, "test-node"),
		mockPacketConner,
		mockQuicConnection,
		mockQuicStream,
		make(chan struct{}),
		&sync.Once{},
		ctx,
	)
}

func runMonitorStatusStreamErrorTest(t *testing.T, failWrite bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	wu, mockBaseWorkUnit, mockNetceptor, w, ctrl := createRemoteWorkTestSetup(t, ctx)

	remoteExtraData := &workceptor.RemoteExtraData{
		RemoteStarted:  true,
		RemoteNode:     "execution",
		RemoteUnitID:   "remoteunit",
		RemoteWorkType: "echoint",
		RemoteParams:   make(map[string]string),
	}
	recorder := &statusDetailRecorder{}

	// monitorRemoteStdout cancels the shared job context if it cannot open the stdout file,
	// which would stop the status goroutine before it writes, so the dir must exist.
	unitDir := t.TempDir()
	expectRemoteUnitStatusSources(mockBaseWorkUnit, w, remoteExtraData, workceptor.StatusFileData{})
	mockBaseWorkUnit.EXPECT().Load().Return(nil).AnyTimes()
	mockBaseWorkUnit.EXPECT().StdoutFileName().Return(filepath.Join(unitDir, "stdout")).AnyTimes()
	mockBaseWorkUnit.EXPECT().UnitDir().Return(unitDir).AnyTimes()
	mockBaseWorkUnit.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockBaseWorkUnit.EXPECT().LastUpdateError().Return(nil).AnyTimes()
	mockBaseWorkUnit.EXPECT().UpdateFullStatus(gomock.Any()).Do(func(updateFunc interface{}) {
		recorder.apply(updateFunc, remoteExtraData)
	}).AnyTimes()

	mockConn := newStatusStreamErrorConn(ctx, ctrl, failWrite)
	mockNetceptor.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&tls.Config{}, nil).AnyTimes()
	mockNetceptor.EXPECT().DialContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockConn, nil).AnyTimes()

	if err := wu.Restart(); err != nil {
		t.Fatalf("Unexpected error restarting work unit: %v", err)
	}

	want := "Connection to execution failed"
	if !waitForDetail(recorder, want) {
		t.Errorf("Expected detail to contain %q, got: %q", want, recorder.get())
	}
}

func TestRemoteWorkMonitorSurfacesStatusStreamErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		failWrite bool
	}{
		{name: "status write error", failWrite: true},
		{name: "status read error", failWrite: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runMonitorStatusStreamErrorTest(t, tt.failWrite)
		})
	}
}

func TestGetConnectionClearsStaleDetailOnSuccess(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wu, mockBaseWorkUnit, mockNetceptor, w, ctrl := createRemoteWorkTestSetup(t, ctx)

	remoteExtraData := &workceptor.RemoteExtraData{
		RemoteNode:     "execution",
		TLSClient:      "test-client",
		RemoteWorkType: "test-work",
		RemoteParams:   make(map[string]string),
	}

	statusLock := &sync.RWMutex{}
	detail := "Connection to execution failed: connection refused"

	mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).AnyTimes()
	mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().DoAndReturn(func() *workceptor.StatusFileData {
		return &workceptor.StatusFileData{}
	}).AnyTimes()
	mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
		ExtraData: remoteExtraData,
	}).AnyTimes()
	mockBaseWorkUnit.EXPECT().GetWorkceptor().Return(w).AnyTimes()
	mockBaseWorkUnit.EXPECT().UpdateFullStatus(gomock.Any()).Do(func(updateFunc interface{}) {
		status := &workceptor.StatusFileData{
			Detail:    detail,
			ExtraData: remoteExtraData,
		}
		updateFunc.(func(*workceptor.StatusFileData))(status)
		detail = status.Detail
	}).AnyTimes()

	mockPacketConner := mock_netceptor.NewMockPacketConner(ctrl)
	mockQuicConnection := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	mockQuicStream := mock_netceptor.NewMockQuicStreamForConn(ctrl)

	helloSent := false
	mockQuicStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(b []byte) (int, error) {
		if !helloSent {
			helloSent = true
			msg := "execution\n"
			copy(b, msg)

			return len(msg), nil
		}

		return 0, ctx.Err()
	}).AnyTimes()
	mockQuicStream.EXPECT().Write(gomock.Any()).Return(0, nil).AnyTimes()
	mockQuicStream.EXPECT().Close().Return(nil).AnyTimes()
	mockQuicStream.EXPECT().SetReadDeadline(gomock.Any()).Return(nil).AnyTimes()
	mockQuicStream.EXPECT().SetWriteDeadline(gomock.Any()).Return(nil).AnyTimes()
	mockQuicStream.EXPECT().SetDeadline(gomock.Any()).Return(nil).AnyTimes()

	var cancelFunc context.CancelFunc = func() {}
	mockPacketConner.EXPECT().Cancel().Return(&cancelFunc).AnyTimes()
	mockPacketConner.EXPECT().Close().Return(nil).AnyTimes()
	mockPacketConner.EXPECT().LocalService().Return("test-service").AnyTimes()
	mockPacketConner.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()

	mockQuicConnection.EXPECT().CloseWithError(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockQuicConnection.EXPECT().RemoteAddr().Return(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}).AnyTimes()
	mockQuicConnection.EXPECT().LocalAddr().Return(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}).AnyTimes()

	mockConn := netceptor.NewConn(
		netceptor.New(ctx, "test-node"),
		mockPacketConner,
		mockQuicConnection,
		mockQuicStream,
		make(chan struct{}),
		&sync.Once{},
		ctx,
	)

	mockNetceptor.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&tls.Config{}, nil).AnyTimes()
	mockNetceptor.EXPECT().DialContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockConn, nil)

	conn, _ := wu.(interface {
		GetConnection(context.Context) (net.Conn, *bufio.Reader)
	}).GetConnection(ctx)

	if conn == nil {
		t.Fatal("Expected non-nil connection")
	}
	if detail != "" {
		t.Errorf("Expected detail to be cleared, got: %s", detail)
	}
}

type remoteWorkAdoptedUnitTestCase struct {
	name                        string
	adopted                     bool
	remoteState                 int
	remoteWorkType              string
	remoteStatusWorkType        string
	lastUpdateError             error
	expectRemoteWorkTypePersist bool
	expectFail                  bool
}

type remoteWorkAdoptedUnitObservations struct {
	failed                   chan workceptor.StatusFileData
	remoteWorkTypePersisted  chan string
	statusWriteErrorObserved chan struct{}
	copied                   chan int
}

// TestRemoteWorkAdoptedUnitNotStarted verifies that an adopted work unit whose remote
// reports a pending state is failed locally, since adoption never submits the work and
// nothing will ever start it. Non-adopted units must keep tracking pending normally.
func TestRemoteWorkAdoptedUnitNotStarted(t *testing.T) {
	t.Parallel()
	tests := []remoteWorkAdoptedUnitTestCase{
		{
			name:                 "adopted_remote_pending_fails_locally",
			adopted:              true,
			remoteState:          workceptor.WorkStatePending,
			remoteStatusWorkType: "echoint",
			expectFail:           true,
		},
		{
			name:                 "adopted_remote_running_keeps_monitoring",
			adopted:              true,
			remoteState:          workceptor.WorkStateRunning,
			remoteWorkType:       "echoint",
			remoteStatusWorkType: "echoint",
		},
		{
			name:                        "adopted_remote_running_persists_missing_work_type",
			adopted:                     true,
			remoteState:                 workceptor.WorkStateRunning,
			remoteStatusWorkType:        "echoint",
			expectRemoteWorkTypePersist: true,
		},
		{
			name:                 "adopted_remote_running_without_reported_work_type",
			adopted:              true,
			remoteState:          workceptor.WorkStateRunning,
			remoteStatusWorkType: "",
		},
		{
			name:                 "submitted_remote_pending_keeps_monitoring",
			remoteState:          workceptor.WorkStatePending,
			remoteWorkType:       "echoint",
			remoteStatusWorkType: "echoint",
		},
		{
			name:                 "submitted_remote_running_retries_status_write",
			remoteState:          workceptor.WorkStateRunning,
			remoteWorkType:       "echoint",
			remoteStatusWorkType: "echoint",
			lastUpdateError:      fmt.Errorf("temporary status write failure"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runRemoteWorkAdoptedUnitTest(t, tt) })
	}
}

func runRemoteWorkAdoptedUnitTest(t *testing.T, tt remoteWorkAdoptedUnitTestCase) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	wu, mockBaseWorkUnit, mockNetceptor, w, ctrl := createRemoteWorkTestSetup(t, ctx)
	t.Cleanup(func() {
		cancel()
		time.Sleep(200 * time.Millisecond)
	})

	remoteExtraData := &workceptor.RemoteExtraData{
		RemoteStarted:  true,
		Adopted:        tt.adopted,
		RemoteNode:     "execution",
		RemoteUnitID:   "remoteunit",
		RemoteWorkType: tt.remoteWorkType,
		RemoteParams:   make(map[string]string),
	}
	observations := setupRemoteWorkAdoptedObservations(t, tt, mockBaseWorkUnit, w, remoteExtraData)
	messages := []string{
		"execution\n",
		fmt.Sprintf("{\"State\": %d, \"Detail\": \"status detail\", \"StdoutSize\": 0, \"WorkType\": %q}\n", tt.remoteState, tt.remoteStatusWorkType),
	}
	createRemoteWorkNetworkSetup(t, ctrl, ctx, messages, mockNetceptor, mockBaseWorkUnit, filepath.Join("/tmp", tt.name), remoteExtraData, true)
	if err := wu.Restart(); err != nil {
		t.Fatalf("Unexpected error restarting work unit: %v", err)
	}
	assertRemoteWorkAdoptedObservations(t, tt, observations)
}

func setupRemoteWorkAdoptedObservations(t *testing.T, tt remoteWorkAdoptedUnitTestCase, mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, w *workceptor.Workceptor, remoteExtraData *workceptor.RemoteExtraData) *remoteWorkAdoptedUnitObservations {
	t.Helper()
	expectRemoteUnitStatusSources(mockBaseWorkUnit, w, remoteExtraData, workceptor.StatusFileData{})

	observations := &remoteWorkAdoptedUnitObservations{
		failed:                   make(chan workceptor.StatusFileData, 1),
		remoteWorkTypePersisted:  make(chan string, 1),
		statusWriteErrorObserved: make(chan struct{}, 1),
		copied:                   make(chan int, 1),
	}
	if tt.lastUpdateError != nil {
		expectRemoteStatusWriteError(mockBaseWorkUnit, tt.lastUpdateError, observations.statusWriteErrorObserved)
	}
	mockBaseWorkUnit.EXPECT().UpdateFullStatus(gomock.Any()).Do(func(updateFunc interface{}) {
		status := workceptor.StatusFileData{ExtraData: remoteExtraData}
		updateFunc.(func(*workceptor.StatusFileData))(&status)
		observeRemoteAdoptedStatus(observations, status)
	}).AnyTimes()
	mockBaseWorkUnit.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(state int, _ string, _ int64) {
		observeRemoteAdoptedState(observations, state)
	}).AnyTimes()

	return observations
}

func expectRemoteStatusWriteError(mockBaseWorkUnit *mock_workceptor.MockBaseWorkUnitForWorkUnit, writeErr error, observed chan struct{}) {
	mockBaseWorkUnit.EXPECT().LastUpdateError().DoAndReturn(func() error {
		observed <- struct{}{}

		return writeErr
	}).Times(1)
}

func observeRemoteAdoptedStatus(observations *remoteWorkAdoptedUnitObservations, status workceptor.StatusFileData) {
	if status.State == workceptor.WorkStateFailed {
		select {
		case observations.failed <- status:
		default:
		}
	}
	extra, ok := status.ExtraData.(*workceptor.RemoteExtraData)
	if ok && extra.RemoteWorkType != "" {
		select {
		case observations.remoteWorkTypePersisted <- extra.RemoteWorkType:
		default:
		}
	}
}

func observeRemoteAdoptedState(observations *remoteWorkAdoptedUnitObservations, state int) {
	select {
	case observations.copied <- state:
	default:
	}
}

func assertRemoteWorkAdoptedObservations(t *testing.T, tt remoteWorkAdoptedUnitTestCase, observations *remoteWorkAdoptedUnitObservations) {
	if tt.expectFail {
		assertRemoteWorkAdoptionFailure(t, observations)

		return
	}
	assertRemoteWorkStatusCopied(t, tt.remoteState, observations.copied)
	assertRemoteWorkStatusWriteErrorObserved(t, tt.lastUpdateError, observations.statusWriteErrorObserved)
	assertRemoteWorkWasNotFailed(t, observations.failed)
	assertRemoteWorkTypePersisted(t, tt.expectRemoteWorkTypePersist, observations.remoteWorkTypePersisted)
}

func assertRemoteWorkAdoptionFailure(t *testing.T, observations *remoteWorkAdoptedUnitObservations) {
	select {
	case status := <-observations.failed:
		if status.Detail != "Adopted remote work unit was never started" {
			t.Errorf("Expected detail 'Adopted remote work unit was never started', got: %s", status.Detail)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Expected adopted work unit reporting pending to be failed locally")
	}
	assertRemoteAdoptedPendingWasNotCopied(t, observations.copied)
}

func assertRemoteAdoptedPendingWasNotCopied(t *testing.T, copied <-chan int) {
	select {
	case state := <-copied:
		t.Errorf("Expected no status copy from a never-started adopted unit, got state %d", state)
	default:
	}
}

func assertRemoteWorkStatusCopied(t *testing.T, wantState int, copied <-chan int) {
	select {
	case state := <-copied:
		if state != wantState {
			t.Errorf("Expected remote state %d to be copied locally, got %d", wantState, state)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Expected remote status to be copied to the local work unit")
	}
}

func assertRemoteWorkStatusWriteErrorObserved(t *testing.T, expected error, observed <-chan struct{}) {
	if expected == nil {
		return
	}
	select {
	case <-observed:
	case <-time.After(10 * time.Second):
		t.Fatal("status write error was not observed")
	}
}

func assertRemoteWorkWasNotFailed(t *testing.T, failed <-chan workceptor.StatusFileData) {
	select {
	case status := <-failed:
		t.Errorf("Unexpected local failure: %s", status.Detail)
	default:
	}
}

func assertRemoteWorkTypePersisted(t *testing.T, expected bool, persisted <-chan string) {
	if !expected {
		return
	}
	select {
	case persistedType := <-persisted:
		if persistedType != "echoint" {
			t.Errorf("RemoteWorkType = %q, want echoint", persistedType)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("remote work type was not persisted")
	}
}

type scriptedChunk struct {
	data string
	err  error
}

// newScriptedConn returns a mock netceptor connection that answers the node
// handshake and then serves whatever chunks the test supplies.
func newScriptedConn(ctx context.Context, ctrl *gomock.Controller, onWrite func(), onRead func(int)) (*netceptor.Conn, chan scriptedChunk) {
	chunks := make(chan scriptedChunk, 8)
	mockPacketConner := mock_netceptor.NewMockPacketConner(ctrl)
	mockQuicConnection := mock_netceptor.NewMockQuicConnectionForConn(ctrl)
	mockQuicStream := mock_netceptor.NewMockQuicStreamForConn(ctrl)

	reads := 0
	mockQuicStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(b []byte) (int, error) {
		if reads == 0 {
			reads++
			copy(b, "execution\n")

			return len("execution\n"), nil
		}
		if onRead != nil {
			onRead(reads)
		}
		reads++
		chunk := <-chunks
		if chunk.err != nil {
			return 0, chunk.err
		}
		copy(b, chunk.data)

		return len(chunk.data), nil
	}).AnyTimes()
	mockQuicStream.EXPECT().Write(gomock.Any()).DoAndReturn(func(b []byte) (int, error) {
		if onWrite != nil {
			onWrite()
		}

		return len(b), nil
	}).AnyTimes()
	mockQuicStream.EXPECT().Close().Return(nil).AnyTimes()
	mockQuicStream.EXPECT().SetReadDeadline(gomock.Any()).Return(nil).AnyTimes()
	mockQuicStream.EXPECT().SetWriteDeadline(gomock.Any()).Return(nil).AnyTimes()
	mockQuicStream.EXPECT().SetDeadline(gomock.Any()).Return(nil).AnyTimes()

	var cancelFunc context.CancelFunc = func() {}
	mockPacketConner.EXPECT().Cancel().Return(&cancelFunc).AnyTimes()
	mockPacketConner.EXPECT().Close().Return(nil).AnyTimes()
	mockPacketConner.EXPECT().LocalService().Return("test-service").AnyTimes()
	mockPacketConner.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()

	mockQuicConnection.EXPECT().CloseWithError(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockQuicConnection.EXPECT().RemoteAddr().Return(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}).AnyTimes()
	mockQuicConnection.EXPECT().LocalAddr().Return(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}).AnyTimes()

	return netceptor.NewConn(
		netceptor.New(ctx, "test-node"),
		mockPacketConner,
		mockQuicConnection,
		mockQuicStream,
		make(chan struct{}),
		&sync.Once{},
		ctx,
	), chunks
}

// newRemoteMonitorTestUnit builds a real base unit behind a remote worker so the
// released flag and status writes behave as they do in production.
func newRemoteMonitorTestUnit(t *testing.T, ctx context.Context, ctrl *gomock.Controller, logBuf *bytes.Buffer) (workceptor.WorkUnit, *workceptor.BaseWorkUnit, *mock_workceptor.MockNetceptorForWorkceptor) {
	t.Helper()
	mockNC := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
	mockNC.EXPECT().NodeID().Return("testnode").AnyTimes()
	lg := logger.NewReceptorLogger("")
	lg.SetOutput(logBuf)
	mockNC.EXPECT().GetLogger().Return(lg).AnyTimes()

	w, err := workceptor.New(ctx, mockNC, t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	bwu := &workceptor.BaseWorkUnit{}
	rw := workceptor.NewRemoteWorker(bwu, w, "unit", "remote")
	if err := os.MkdirAll(bwu.UnitDir(), 0o700); err != nil {
		t.Fatalf("mkdir unit dir: %v", err)
	}
	bwu.SetStatusExtraData(&workceptor.RemoteExtraData{
		RemoteStarted:  true,
		RemoteNode:     "execution",
		RemoteUnitID:   "remoteunit",
		RemoteParams:   map[string]string{},
		RemoteWorkType: "echoint",
	})

	return rw, bwu, mockNC
}

// TestMonitorRemoteStatusStopsWhenUnitReleased covers the ErrUnitReleased guard:
// a status write after the local proxy directory was removed must stop the
// monitor instead of counting retries.
func TestMonitorRemoteStatusStopsWhenUnitReleased(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctrl := gomock.NewController(t)
	logBuf := &bytes.Buffer{}
	_, bwu, mockNC := newRemoteMonitorTestUnit(t, ctx, ctrl, logBuf)

	statusRequested := make(chan struct{})
	var once sync.Once
	conn, chunks := newScriptedConn(ctx, ctrl, func() { once.Do(func() { close(statusRequested) }) }, nil)
	mockNC.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&tls.Config{}, nil).AnyTimes()
	mockNC.EXPECT().DialContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(conn, nil).AnyTimes()

	mw := &utils.JobContext{}
	mw.NewJob(ctx, 1, false)
	done := make(chan struct{})
	go func() {
		workceptor.MonitorRemoteStatusForTest(bwu, mw, false)
		close(done)
	}()

	select {
	case <-statusRequested:
	case <-time.After(5 * time.Second):
		t.Fatal("status monitor never sent a status request")
	}

	if err := bwu.Release(false); err != nil {
		t.Fatalf("Release: %v", err)
	}
	chunks <- scriptedChunk{data: "{\"State\":1,\"Detail\":\"running\",\"StdoutSize\":0}\n"}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("status monitor did not stop after release")
	}
	if strings.Contains(logBuf.String(), "Error updating status file") {
		t.Fatalf("release logged a status update error:\n%s", logBuf.String())
	}
}

// TestMonitorRemoteStdoutStopsQuietlyAfterRelease covers the copy-error guard:
// a read cancelled by release must not log a warning.
func TestMonitorRemoteStdoutStopsQuietlyAfterRelease(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl := gomock.NewController(t)
	logBuf := &bytes.Buffer{}
	_, bwu, mockNC := newRemoteMonitorTestUnit(t, ctx, ctrl, logBuf)

	bwu.UpdateFullStatus(func(s *workceptor.StatusFileData) {
		s.State = workceptor.WorkStateRunning
		s.StdoutSize = 1
	})
	if err := os.WriteFile(bwu.StdoutFileName(), nil, 0o600); err != nil {
		t.Fatalf("write stdout: %v", err)
	}

	resultsRequested := make(chan struct{})
	copying := make(chan struct{})
	var resultsOnce, copyOnce sync.Once
	conn, chunks := newScriptedConn(ctx, ctrl,
		func() { resultsOnce.Do(func() { close(resultsRequested) }) },
		func(reads int) {
			if reads == 2 {
				copyOnce.Do(func() { close(copying) })
			}
		})
	mockNC.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&tls.Config{}, nil).AnyTimes()
	mockNC.EXPECT().DialContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(conn, nil).AnyTimes()

	mw := &utils.JobContext{}
	mw.NewJob(ctx, 1, false)
	done := make(chan struct{})
	go func() {
		workceptor.MonitorRemoteStdoutForTest(bwu, mw)
		close(done)
	}()

	select {
	case <-resultsRequested:
	case <-time.After(5 * time.Second):
		t.Fatal("stdout monitor never requested results")
	}
	chunks <- scriptedChunk{data: "Streaming results\n"}
	select {
	case <-copying:
	case <-time.After(5 * time.Second):
		t.Fatal("stdout monitor never started copying")
	}

	if err := bwu.Release(false); err != nil {
		t.Fatalf("Release: %v", err)
	}
	chunks <- scriptedChunk{err: errors.New("error code 499")}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("stdout monitor did not stop after release")
	}
	if strings.Contains(logBuf.String(), "Could not copy to stdout file") {
		t.Fatalf("release logged a stdout copy warning:\n%s", logBuf.String())
	}
}

// TestMonitorRemoteStdoutQuietWhenReleasedBeforeOpen covers the open-error guard:
// a release before the monitor opens stdout must not log an error.
func TestMonitorRemoteStdoutQuietWhenReleasedBeforeOpen(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctrl := gomock.NewController(t)
	logBuf := &bytes.Buffer{}
	_, bwu, _ := newRemoteMonitorTestUnit(t, ctx, ctrl, logBuf)

	bwu.UpdateFullStatus(func(s *workceptor.StatusFileData) {
		s.State = workceptor.WorkStateRunning
	})
	if err := bwu.Release(false); err != nil {
		t.Fatalf("Release: %v", err)
	}

	mw := &utils.JobContext{}
	mw.NewJob(ctx, 1, false)
	done := make(chan struct{})
	go func() {
		workceptor.MonitorRemoteStdoutForTest(bwu, mw)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stdout monitor did not stop after release")
	}
	if strings.Contains(logBuf.String(), "Could not open stdout file") {
		t.Fatalf("release logged a stdout open error:\n%s", logBuf.String())
	}
}

// TestMonitorRemoteStdoutQuietWhenReleasedDuringResultsHandshake covers the
// second open guard: a release that lands after the local stdout file was opened
// but before the results handshake completes must not log an error when the
// streaming path reopens stdout.
func TestMonitorRemoteStdoutQuietWhenReleasedDuringResultsHandshake(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl := gomock.NewController(t)
	logBuf := &bytes.Buffer{}
	_, bwu, mockNC := newRemoteMonitorTestUnit(t, ctx, ctrl, logBuf)

	bwu.UpdateFullStatus(func(s *workceptor.StatusFileData) {
		s.State = workceptor.WorkStateRunning
		s.StdoutSize = 1
	})
	if err := os.WriteFile(bwu.StdoutFileName(), nil, 0o600); err != nil {
		t.Fatalf("write stdout: %v", err)
	}

	resultsRequested := make(chan struct{})
	var resultsOnce sync.Once
	conn, chunks := newScriptedConn(ctx, ctrl,
		func() { resultsOnce.Do(func() { close(resultsRequested) }) },
		nil)
	mockNC.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&tls.Config{}, nil).AnyTimes()
	mockNC.EXPECT().DialContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(conn, nil).AnyTimes()

	mw := &utils.JobContext{}
	mw.NewJob(ctx, 1, false)
	done := make(chan struct{})
	go func() {
		workceptor.MonitorRemoteStdoutForTest(bwu, mw)
		close(done)
	}()

	select {
	case <-resultsRequested:
	case <-time.After(5 * time.Second):
		t.Fatal("stdout monitor never requested results")
	}

	// Release while the handshake is in flight: the local stdout file has already
	// been opened once, and the streaming path is about to open it again.
	if err := bwu.Release(false); err != nil {
		t.Fatalf("Release: %v", err)
	}
	chunks <- scriptedChunk{data: "Streaming results\n"}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("stdout monitor did not stop after release")
	}
	if strings.Contains(logBuf.String(), "Could not open stdout file") {
		t.Fatalf("release logged a stdout open error:\n%s", logBuf.String())
	}
}

const (
	logTestLocalNode  = "node1"
	logTestRemoteNode = "node2"
)

// logRemoteID returns the structured remote_id suffix the handshake assigns to
// the shared logger.
func logRemoteID(node string) string {
	return `"remote_id":"` + node + `"`
}

// connectTwoNetceptorNodes creates two real Netceptor nodes connected via
// external backends over a Unix socket pair, and returns the local node with
// its log output captured. After the handshake, the local node's shared logger
// carries the remote peer's identity, so log lines that go through
// rw.GetWorkceptor().nc.GetLogger() are stamped with it without any logger
// being constructed by the test.
func connectTwoNetceptorNodes(t *testing.T, ctx context.Context, localNode, remoteNode string) (*netceptor.Netceptor, *netceptor.Netceptor, *testutils.LogCapture) {
	t.Helper()

	n1 := netceptor.New(ctx, localNode)
	logCapture := testutils.NewLogCapture()
	n1.Logger.SetOutput(logCapture)

	n2 := netceptor.New(ctx, remoteNode)
	n2.Logger.SetOutput(io.Discard)

	b1, err := netceptor.NewExternalBackend()
	if err != nil {
		t.Fatal(err)
	}
	if err := n1.AddBackend(b1); err != nil {
		t.Fatal(err)
	}

	b2, err := netceptor.NewExternalBackend()
	if err != nil {
		t.Fatal(err)
	}
	if err := n2.AddBackend(b2); err != nil {
		t.Fatal(err)
	}

	// Subscribe before connecting: the initial routing-update flood fires
	// as soon as the backends are linked, and the next one only lands at
	// defaultRouteUpdateTime, which would exhaust the timeout below.
	routesChan := n1.SubscribeRoutingUpdates()

	c1, c2, err := socketpair.New("unix")
	if err != nil {
		t.Fatal(err)
	}
	b1.NewConnection(netceptor.MessageConnFromNetConn(c1), true)
	b2.NewConnection(netceptor.MessageConnFromNetConn(c2), true)
	t.Cleanup(func() {
		_ = c1.Close()
		_ = c2.Close()
	})
	t.Cleanup(n1.Shutdown)
	t.Cleanup(n2.Shutdown)

	// Wait for the local node to establish routing to the remote node.
	timeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		select {
		case <-timeout.Done():
			t.Fatalf("timed out waiting for %s to connect to %s; logs:\n%s", localNode, remoteNode, logCapture.String())
		case routes, ok := <-routesChan:
			if !ok {
				t.Fatalf("routing update subscription closed before %s connected to %s; logs:\n%s", localNode, remoteNode, logCapture.String())
			}
			if routes != nil {
				if _, ok := routes[remoteNode]; ok {
					return n1, n2, logCapture
				}
			}
		}
	}
}

// startFakeControlListener advertises a "control" service on n2 and hands each
// accepted connection to handle.
func startFakeControlListener(t *testing.T, n2 *netceptor.Netceptor, handle func(net.Conn)) {
	t.Helper()
	listener, err := n2.ListenAndAdvertise("control", nil, nil)
	if err != nil {
		t.Fatalf("could not listen on control service: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				// A dialer that closes before streaming its init byte
				// surfaces here as a per-connection error; keep serving.
				select {
				case <-listener.DoneChan:
					return
				default:
					continue
				}
			}
			go handle(conn)
		}
	}()
}

// startFakeControlService advertises a "control" service on n2 that greets the
// caller with the node ID and delegates each request line to handle. handle
// returns false to close the connection after the request.
func startFakeControlService(t *testing.T, n2 *netceptor.Netceptor, node string, handle func(conn net.Conn, req string) bool) {
	t.Helper()
	startFakeControlListener(t, n2, func(conn net.Conn) {
		defer conn.Close()
		if _, err := fmt.Fprintf(conn, "%s\n", node); err != nil {
			return
		}
		reader := bufio.NewReader(conn)
		for {
			req, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if !handle(conn, strings.TrimSpace(req)) {
				return
			}
		}
	})
}

// waitForControlDial dials until the control service is reachable.
func waitForControlDial(t *testing.T, n1 *netceptor.Netceptor, node string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var lastErr error
	ready := testutils.CheckUntilTimeout(ctx, 50*time.Millisecond, func() bool {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), time.Second)
		defer dialCancel()
		conn, err := n1.DialContext(dialCtx, node, "control", nil)
		if err != nil {
			lastErr = err

			return false
		}
		_ = conn.CloseConnection()

		return true
	})
	if !ready {
		t.Fatalf("timed out waiting for control service on %s: %v", node, lastErr)
	}
}

// waitForControlService waits for the advertisement to propagate and verifies
// that the service greets the caller with the node ID.
func waitForControlService(t *testing.T, n1 *netceptor.Netceptor, node string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var lastErr error
	greeted := testutils.CheckUntilTimeout(ctx, 50*time.Millisecond, func() bool {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), time.Second)
		defer dialCancel()
		conn, err := n1.DialContext(dialCtx, node, "control", nil)
		if err != nil {
			lastErr = err

			return false
		}
		defer func() { _ = conn.CloseConnection() }()
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		hello, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			lastErr = err

			return false
		}
		if !strings.Contains(hello, node) {
			lastErr = fmt.Errorf("control service on %s greeted with %q", node, strings.TrimSpace(hello))

			return false
		}

		return true
	})
	if !greeted {
		t.Fatalf("timed out waiting for control service on %s: %v", node, lastErr)
	}
}

// newRemoteCloseTestUnit builds a remote unit owned by a Workceptor backed by
// the real n1 logger, so the assertions below observe the remote_id set during
// the two-node handshake. It returns the mocked base unit and its unit dir.
func newRemoteCloseTestUnit(t *testing.T, ctx context.Context, n1 *netceptor.Netceptor, status workceptor.StatusFileData) (*mock_workceptor.MockBaseWorkUnitForWorkUnit, string) {
	t.Helper()
	ctrl := gomock.NewController(t)
	base := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
	w, err := workceptor.New(ctx, n1, t.TempDir())
	if err != nil {
		t.Fatalf("could not create workceptor: %v", err)
	}
	t.Cleanup(w.Cancel)

	red := &workceptor.RemoteExtraData{
		RemoteNode:   logTestRemoteNode,
		RemoteUnitID: "remote-unit",
		RemoteParams: make(map[string]string),
	}
	unitDir := t.TempDir()
	expectRemoteUnitStatusSources(base, w, red, status)
	base.EXPECT().UpdateFullStatus(gomock.Any()).AnyTimes()
	base.EXPECT().Load().Return(nil).AnyTimes()
	base.EXPECT().UnitDir().Return(unitDir).AnyTimes()
	base.EXPECT().StdoutFileName().Return(filepath.Join(unitDir, "stdout")).AnyTimes()

	return base, unitDir
}

// TestRemoteWorkStdinCloseErrorLogCarriesRemoteID pins the "Error closing"
// line emitted by StartRemoteUnit when the stdin handle fails to close. The
// message names only the local file, so the remote_id suffix is the only
// carrier of the peer's identity.
func TestRemoteWorkStdinCloseErrorLogCarriesRemoteID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	n1, n2, logCapture := connectTwoNetceptorNodes(t, ctx, logTestLocalNode, logTestRemoteNode)

	// Fake control service that completes the submit handshake: greeting,
	// submit command, ID response, stdin until EOF, then OK.
	startFakeControlListener(t, n2, func(conn net.Conn) {
		defer conn.Close()
		if _, err := fmt.Fprintf(conn, "%s\n", logTestRemoteNode); err != nil {
			return
		}
		reader := bufio.NewReader(conn)
		if _, err := reader.ReadString('\n'); err != nil {
			return
		}
		if _, err := io.WriteString(conn, "Work unit created with ID unit1. Send stdin data and EOF.\n"); err != nil {
			return
		}
		if _, err := io.Copy(io.Discard, reader); err != nil {
			return
		}
		_, _ = io.WriteString(conn, "OK\n")
	})
	waitForControlService(t, n1, logTestRemoteNode)

	mockBaseWorkUnit, unitDir := newRemoteCloseTestUnit(t, ctx, n1, workceptor.StatusFileData{State: workceptor.WorkStateRunning})

	// StartRemoteUnit opens <UnitDir>/stdin, so it must exist.
	if err := os.WriteFile(filepath.Join(unitDir, "stdin"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	restore := workceptor.SetCloseFileForTest(func(*os.File) error { return errors.New("close boom") })
	t.Cleanup(restore)

	dialCtx, dialCancel := context.WithTimeout(ctx, 5*time.Second)
	defer dialCancel()
	conn, err := n1.DialContext(dialCtx, logTestRemoteNode, "control", nil)
	if err != nil {
		t.Fatalf("could not dial control service: %v", err)
	}
	reader := bufio.NewReader(conn)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("could not read control greeting: %v", err)
	}

	if err := workceptor.StartRemoteUnitForTest(mockBaseWorkUnit, ctx, conn, reader); err != nil {
		t.Fatalf("StartRemoteUnit failed: %v", err)
	}

	logCapture.WaitFor(t, 5*time.Second, "Error closing", "stdin", logRemoteID(logTestRemoteNode))
}

// TestRemoteWorkStdoutCloseErrorLogCarriesRemoteID pins the "Error closing"
// line emitted by monitorRemoteStdout when the stdout handle fails to close.
func TestRemoteWorkStdoutCloseErrorLogCarriesRemoteID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	n1, n2, logCapture := connectTwoNetceptorNodes(t, ctx, logTestLocalNode, logTestRemoteNode)

	// StdoutSize matches the streamed payload so that, once it is copied, the
	// next loop iteration sees a complete unit and returns, running the
	// deferred close.
	status := workceptor.StatusFileData{State: workceptor.WorkStateSucceeded, StdoutSize: int64(len("partial"))}
	startFakeControlService(t, n2, logTestRemoteNode, func(conn net.Conn, _ string) bool {
		_, _ = io.WriteString(conn, "Streaming results\n")
		_, _ = io.WriteString(conn, "partial")

		return false
	})
	waitForControlService(t, n1, logTestRemoteNode)

	mockBaseWorkUnit, _ := newRemoteCloseTestUnit(t, ctx, n1, status)

	restore := workceptor.SetCloseFileForTest(func(*os.File) error { return errors.New("close boom") })
	t.Cleanup(restore)

	job := &utils.JobContext{}
	job.NewJob(ctx, 1, false)
	go workceptor.MonitorRemoteStdoutForTest(mockBaseWorkUnit, job)
	job.Wait()

	logCapture.WaitFor(t, 5*time.Second, "Error closing", "stdout", logRemoteID(logTestRemoteNode))
}

// newRemoteLogTestEnv builds a Workceptor backed by the handshake-suffixed
// n1 logger, plus a mocked base work unit whose status is controlled by the
// caller. Because w.nc is the real n1, every rw.GetWorkceptor().nc.GetLogger()
// call observes the remote_id set during the handshake.
func newRemoteLogTestEnv(t *testing.T, ctx context.Context, n1 *netceptor.Netceptor, red *workceptor.RemoteExtraData, status workceptor.StatusFileData, unitDir string, lastUpdateError error) (*workceptor.Workceptor, *mock_workceptor.MockBaseWorkUnitForWorkUnit) {
	t.Helper()

	ctrl := gomock.NewController(t)
	base := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
	w, err := workceptor.New(ctx, n1, t.TempDir())
	if err != nil {
		t.Fatalf("could not create workceptor: %v", err)
	}
	t.Cleanup(w.Cancel)

	expectRemoteUnitStatusSources(base, w, red, status)
	base.EXPECT().LastUpdateError().Return(lastUpdateError).AnyTimes()
	if unitDir != "" {
		base.EXPECT().UnitDir().Return(unitDir).AnyTimes()
		base.EXPECT().StatusFileName().Return(filepath.Join(unitDir, "status")).AnyTimes()
	}
	base.EXPECT().UpdateBasicStatus(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	base.EXPECT().UpdateFullStatus(gomock.Any()).Do(func(update func(*workceptor.StatusFileData)) {
		update(&workceptor.StatusFileData{ExtraData: red})
	}).AnyTimes()

	return w, base
}

// remoteMonitorOptions configures a remote monitor test environment.
type remoteMonitorOptions struct {
	// remote is the unit's remote extra data, and status its initial status.
	remote *workceptor.RemoteExtraData
	status workceptor.StatusFileData
	// unitDir, when set, is where the unit's status and stdout files live.
	unitDir string
	// lastUpdateError is what the mocked LastUpdateError returns.
	lastUpdateError error
	// service, when non-nil, advertises a fake control service on the remote
	// node and handles each request line with it.
	service func(conn net.Conn, req string) bool
	// dialOnly skips the hello check, for fake services that do not greet.
	dialOnly bool
}

// remoteMonitorEnv is the shared fixture for remote monitor logging tests: the
// two connected nodes, the log capture, the mocked unit and its job context.
type remoteMonitorEnv struct {
	ctx     context.Context
	capture *testutils.LogCapture
	unit    *mock_workceptor.MockBaseWorkUnitForWorkUnit
	job     *utils.JobContext
}

// newRemoteMonitorEnv connects the two nodes, optionally starts a fake control
// service, builds the mocked unit and creates a job context for the monitor.
func newRemoteMonitorEnv(t *testing.T, opts remoteMonitorOptions) *remoteMonitorEnv {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	n1, n2, capture := connectTwoNetceptorNodes(t, ctx, logTestLocalNode, logTestRemoteNode)
	if opts.service != nil {
		startFakeControlService(t, n2, logTestRemoteNode, opts.service)
		if opts.dialOnly {
			waitForControlDial(t, n1, logTestRemoteNode)
		} else {
			waitForControlService(t, n1, logTestRemoteNode)
		}
	}

	_, unit := newRemoteLogTestEnv(t, ctx, n1, opts.remote, opts.status, opts.unitDir, opts.lastUpdateError)
	job := &utils.JobContext{}
	job.NewJob(ctx, 1, false)

	return &remoteMonitorEnv{ctx: ctx, capture: capture, unit: unit, job: job}
}

// startStdout launches monitorRemoteStdout on the shared job context.
func (e *remoteMonitorEnv) startStdout() {
	go workceptor.MonitorRemoteStdoutForTest(e.unit, e.job)
}

// startStatus launches monitorRemoteStatus on the shared job context.
func (e *remoteMonitorEnv) startStatus(forRelease bool) {
	go workceptor.MonitorRemoteStatusForTest(e.unit, e.job, forRelease)
}

// cancel cancels the job context mid-test.
func (e *remoteMonitorEnv) cancel() {
	e.job.Cancel()
}

// wait joins the monitor goroutine.
func (e *remoteMonitorEnv) wait() {
	e.job.Wait()
}

// newRemoteExtraData builds status extra data targeting the remote node.
func newRemoteExtraData() *workceptor.RemoteExtraData {
	return &workceptor.RemoteExtraData{
		RemoteNode:   logTestRemoteNode,
		RemoteUnitID: "remote-unit",
		RemoteParams: make(map[string]string),
	}
}

// TestRemoteWorkStdoutErrorLogCarriesRemoteID verifies that the workceptor
// "Could not open stdout file" error, emitted while monitoring a remote work
// unit, identifies the remote peer via the structured remote_id suffix. The
// message text names only the local file, so the suffix is the only carrier
// of the peer's identity.
func TestRemoteWorkStdoutErrorLogCarriesRemoteID(t *testing.T) {
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{remote: newRemoteExtraData()})

	// A path whose parent is a regular file can never be opened.
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.unit.EXPECT().StdoutFileName().Return(filepath.Join(blocker, "stdout")).AnyTimes()

	env.startStdout()
	env.capture.WaitFor(t, 5*time.Second, "Could not open stdout file", logRemoteID(logTestRemoteNode))
	// Join the monitor goroutine so it cannot touch the mock after the
	// test (and the gomock controller) has finished.
	env.wait()
}

// TestRemoteWorkStatusFileReadErrorLogCarriesRemoteID pins the
// "Could not read status file" line emitted by monitorRemoteStdout.
func TestRemoteWorkStatusFileReadErrorLogCarriesRemoteID(t *testing.T) {
	unitDir := t.TempDir()
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote:  newRemoteExtraData(),
		status:  workceptor.StatusFileData{State: workceptor.WorkStateRunning},
		unitDir: unitDir,
	})
	env.unit.EXPECT().StdoutFileName().Return(filepath.Join(unitDir, "stdout")).AnyTimes()
	env.unit.EXPECT().Load().Return(errors.New("status unreadable")).AnyTimes()

	env.startStdout()
	env.capture.WaitFor(t, 5*time.Second, "Could not read status file", logRemoteID(logTestRemoteNode))
	env.wait()
}

// TestRemoteWorkRemoteErrorLogCarriesRemoteID pins the "Remote error" line
// emitted by monitorRemoteStatus.
func TestRemoteWorkRemoteErrorLogCarriesRemoteID(t *testing.T) {
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote: newRemoteExtraData(),
		status: workceptor.StatusFileData{State: workceptor.WorkStateRunning},
		service: func(conn net.Conn, _ string) bool {
			_, _ = io.WriteString(conn, "ERROR\tboom\n")

			return false
		},
	})

	env.startStatus(false)
	env.capture.WaitFor(t, 5*time.Second, "Remote error", logRemoteID(logTestRemoteNode))
	env.wait()
}

// TestRemoteWorkStatusUnmarshalErrorLogCarriesRemoteID pins the
// "Error unmarshalling JSON" line emitted by monitorRemoteStatus.
func TestRemoteWorkStatusUnmarshalErrorLogCarriesRemoteID(t *testing.T) {
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote: newRemoteExtraData(),
		status: workceptor.StatusFileData{State: workceptor.WorkStateRunning},
		service: func(conn net.Conn, _ string) bool {
			_, _ = io.WriteString(conn, "not-json\n")

			return false
		},
	})

	env.startStatus(false)
	env.capture.WaitFor(t, 5*time.Second, "Error unmarshalling JSON", logRemoteID(logTestRemoteNode))
	env.wait()
}

// TestRemoteWorkStatusWriteRetryExceededLogCarriesRemoteID pins the
// "Exceeded retries for updating status file" line emitted by
// monitorRemoteStatus.
func TestRemoteWorkStatusWriteRetryExceededLogCarriesRemoteID(t *testing.T) {
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote:          newRemoteExtraData(),
		status:          workceptor.StatusFileData{State: workceptor.WorkStateRunning},
		lastUpdateError: errors.New("status write failed"),
		service: func(conn net.Conn, _ string) bool {
			_, _ = io.WriteString(conn, `{"State":1}`+"\n")

			return true
		},
	})

	env.startStatus(false)
	env.capture.WaitFor(t, 10*time.Second, "Exceeded retries for updating status file", logRemoteID(logTestRemoteNode))
	env.wait()
}

// TestRemoteWorkCryptoErrorDetailLogCarriesRemoteID pins the bare "%s" crypto
// detail line emitted by handleConnectionError.
func TestRemoteWorkCryptoErrorDetailLogCarriesRemoteID(t *testing.T) {
	red := newRemoteExtraData()
	red.RemoteStarted = false
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{remote: red})
	env.job.WorkerDone()

	shouldExit := workceptor.HandleConnectionErrorForTest(env.unit, errors.New("CRYPTO_ERROR: handshake failed"))
	if !shouldExit {
		t.Fatal("expected a crypto error before the remote started to be fatal")
	}

	env.capture.WaitFor(t, 5*time.Second, "TLS error connecting to remote service", logRemoteID(logTestRemoteNode))
}

// TestRemoteWorkCreateSignatureErrorLogCarriesRemoteID pins the
// "could not create signature to get results" line emitted by
// monitorRemoteStdout.
func TestRemoteWorkCreateSignatureErrorLogCarriesRemoteID(t *testing.T) {
	unitDir := t.TempDir()
	red := newRemoteExtraData()
	red.SignWork = true
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote:  red,
		status:  workceptor.StatusFileData{State: workceptor.WorkStateRunning, StdoutSize: 100},
		unitDir: unitDir,
		service: func(_ net.Conn, _ string) bool {
			return true
		},
	})
	env.unit.EXPECT().StdoutFileName().Return(filepath.Join(unitDir, "stdout")).AnyTimes()
	env.unit.EXPECT().Load().Return(nil).AnyTimes()

	env.startStdout()
	env.capture.WaitFor(t, 5*time.Second, "could not create signature to get results", logRemoteID(logTestRemoteNode))
	env.wait()
}

// TestRemoteWorkCopyToStdoutErrorLogCarriesRemoteID pins the
// "Could not copy to stdout file" line emitted by monitorRemoteStdout.
func TestRemoteWorkCopyToStdoutErrorLogCarriesRemoteID(t *testing.T) {
	streamStarted := make(chan struct{})
	release := make(chan struct{})
	unitDir := t.TempDir()
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote:  newRemoteExtraData(),
		status:  workceptor.StatusFileData{State: workceptor.WorkStateRunning, StdoutSize: 1000},
		unitDir: unitDir,
		service: func(conn net.Conn, _ string) bool {
			_, _ = io.WriteString(conn, "Streaming results\n")
			_, _ = io.WriteString(conn, "partial")
			close(streamStarted)
			<-release

			return true
		},
	})
	stdoutPath := filepath.Join(unitDir, "stdout")
	env.unit.EXPECT().StdoutFileName().Return(stdoutPath).AnyTimes()
	env.unit.EXPECT().Load().Return(nil).AnyTimes()

	env.startStdout()

	<-streamStarted
	// Wait until io.Copy has written the streamed bytes before cancelling the
	// read, otherwise the cancellation is observed by the earlier read and the
	// copy error this test pins is never reached.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	if !testutils.CheckUntilTimeout(waitCtx, 5*time.Millisecond, func() bool {
		info, err := os.Stat(stdoutPath)

		return err == nil && info.Size() > 0
	}) {
		t.Fatal("timed out waiting for the stdout copy to start")
	}
	env.cancel()
	close(release)

	env.capture.WaitFor(t, 5*time.Second, "Could not copy to stdout file", logRemoteID(logTestRemoteNode))
	env.wait()
}

// TestRemoteWorkInLoopStdoutOpenErrorLogCarriesRemoteID pins the in-loop
// "Could not open stdout file" line emitted by monitorRemoteStdout.
func TestRemoteWorkInLoopStdoutOpenErrorLogCarriesRemoteID(t *testing.T) {
	unitDir := t.TempDir()
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote:  newRemoteExtraData(),
		status:  workceptor.StatusFileData{State: workceptor.WorkStateRunning, StdoutSize: 1000},
		unitDir: unitDir,
		service: func(conn net.Conn, _ string) bool {
			_, _ = io.WriteString(conn, "Streaming results\n")
			_, _ = io.WriteString(conn, "0123456789")

			return false
		},
	})
	// Calls 1 and 2 are the pre-loop open and the first in-loop open. The
	// third call is the second loop iteration's open, which must fail.
	stdoutCalls := 0
	env.unit.EXPECT().StdoutFileName().DoAndReturn(func() string {
		stdoutCalls++
		if stdoutCalls <= 2 {
			return filepath.Join(unitDir, "stdout")
		}

		return filepath.Join(blocker, "stdout")
	}).AnyTimes()
	env.unit.EXPECT().Load().Return(nil).AnyTimes()

	env.startStdout()
	env.capture.WaitFor(t, 10*time.Second, "Could not open stdout file", logRemoteID(logTestRemoteNode))
	env.wait()

	// The identical message is also emitted by the pre-loop open, so prove the
	// in-loop branch actually ran.
	if stdoutCalls < 3 {
		t.Errorf("expected at least 3 StdoutFileName calls (pre-loop plus two in-loop opens), got %d", stdoutCalls)
	}
}

// TestRemoteWorkActionErrorLogCarriesRemoteID pins the
// "Error running action function" line emitted by getConnectionAndRun.
func TestRemoteWorkActionErrorLogCarriesRemoteID(t *testing.T) {
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote: newRemoteExtraData(),
		service: func(_ net.Conn, _ string) bool {
			return true
		},
	})
	env.job.WorkerDone()

	actionDone := make(chan struct{})
	_ = workceptor.GetConnectionAndRunForTest(env.unit, env.ctx, false, func(context.Context, net.Conn, *bufio.Reader) error {
		defer close(actionDone)

		return errors.New("action boom")
	}, func() {})

	env.capture.WaitFor(t, 5*time.Second, "Error running action function", logRemoteID(logTestRemoteNode))
	// Join the action goroutine so it cannot touch the mock after the test.
	<-actionDone
}

// TestRemoteWorkConnectAndRunErrorLogCarriesRemoteID pins the
// "Error with connect and run" line emitted by cancelOrRelease.
func TestRemoteWorkConnectAndRunErrorLogCarriesRemoteID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n1, n2, logCapture := connectTwoNetceptorNodes(t, ctx, logTestLocalNode, logTestRemoteNode)

	// Reply with a hello naming the wrong node so ConnectToRemote fails
	// immediately on the mismatch instead of waiting out its hello timeout.
	startFakeControlListener(t, n2, func(conn net.Conn) {
		_, _ = io.WriteString(conn, "wrong-node\n")
		_ = conn.Close()
	})
	waitForControlDial(t, n1, logTestRemoteNode)

	red := newRemoteExtraData()
	red.RemoteStarted = true
	w, mockBaseWorkUnit := newRemoteLogTestEnv(t, ctx, n1, red, workceptor.StatusFileData{State: workceptor.WorkStateRunning}, "", nil)
	mockBaseWorkUnit.EXPECT().Init(gomock.Any(), "", "", gomock.Any()).AnyTimes()
	mockBaseWorkUnit.EXPECT().SetStatusExtraData(gomock.Any()).AnyTimes()
	mockBaseWorkUnit.EXPECT().Release(true).Return(nil).AnyTimes()

	unit := workceptor.NewRemoteWorker(mockBaseWorkUnit, w, "", "")
	if err := unit.Release(true); err != nil {
		t.Fatal(err)
	}

	logCapture.WaitFor(t, 5*time.Second, "Error with connect and run", logRemoteID(logTestRemoteNode))
}

// TestRemoteWorkConnectionErrorLogCarriesRemoteID pins the "Connection to %s
// failed" line emitted by handleConnectionError.
func TestRemoteWorkConnectionErrorLogCarriesRemoteID(t *testing.T) {
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{remote: newRemoteExtraData()})
	env.job.WorkerDone()

	_ = workceptor.HandleConnectionErrorForTest(env.unit, errors.New("connection boom"))

	env.capture.WaitFor(t, 5*time.Second, "Connection to", logRemoteID(logTestRemoteNode))
}

// TestRemoteWorkAdoptedNotStartedLogCarriesRemoteID pins the "Adopted work
// unit ... has not started" line emitted by monitorRemoteStatus.
func TestRemoteWorkAdoptedNotStartedLogCarriesRemoteID(t *testing.T) {
	red := newRemoteExtraData()
	red.Adopted = true
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote: red,
		status: workceptor.StatusFileData{State: workceptor.WorkStateRunning},
		service: func(conn net.Conn, _ string) bool {
			_, _ = io.WriteString(conn, `{"State":0}`+"\n")

			return true
		},
	})

	env.startStatus(false)
	env.capture.WaitFor(t, 5*time.Second, "has not started", logRemoteID(logTestRemoteNode))
	env.wait()
}

// TestRemoteWorkDidNotStreamResultsLogCarriesRemoteID pins the "did not stream
// results" line emitted by monitorRemoteStdout when the remote answers the
// results request without announcing a stream.
func TestRemoteWorkDidNotStreamResultsLogCarriesRemoteID(t *testing.T) {
	unitDir := t.TempDir()
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote:  newRemoteExtraData(),
		status:  workceptor.StatusFileData{State: workceptor.WorkStateRunning, StdoutSize: 1000},
		unitDir: unitDir,
		service: func(conn net.Conn, _ string) bool {
			_, _ = io.WriteString(conn, "not streaming\n")

			return true
		},
	})
	env.unit.EXPECT().StdoutFileName().Return(filepath.Join(unitDir, "stdout")).AnyTimes()
	env.unit.EXPECT().Load().Return(nil).AnyTimes()

	env.startStdout()
	env.capture.WaitFor(t, 5*time.Second, "did not stream results", logRemoteID(logTestRemoteNode))
	env.cancel()
	env.wait()
}

// TestRemoteWorkReleaseErrorLogCarriesRemoteID pins the "Error releasing unit"
// line emitted after a successful connection when releasing the unit fails.
func TestRemoteWorkReleaseErrorLogCarriesRemoteID(t *testing.T) {
	unitDir := t.TempDir()
	env := newRemoteMonitorEnv(t, remoteMonitorOptions{
		remote:  newRemoteExtraData(),
		unitDir: unitDir,
		service: func(_ net.Conn, _ string) bool {
			return true
		},
	})
	env.unit.EXPECT().Release(false).Return(errors.New("release boom")).AnyTimes()

	go func() {
		_ = workceptor.RunAndMonitorForTest(env.unit, env.job, true, func(context.Context, net.Conn, *bufio.Reader) error {
			return nil
		})
	}()

	env.capture.WaitFor(t, 5*time.Second, "Error releasing unit", logRemoteID(logTestRemoteNode))
	env.wait()
}
