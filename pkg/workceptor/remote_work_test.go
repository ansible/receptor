package workceptor_test

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
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
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
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
	// Set up reads to return different messages based on the provided list
	readExpectation := mockQuicStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(b []byte) (int, error) {
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

	mockBaseWorkUnit.EXPECT().Init(w, "", "", workceptor.FileSystem{})
	mockBaseWorkUnit.EXPECT().SetStatusExtraData(gomock.Any())
	workUnit := workceptor.NewRemoteWorker(mockBaseWorkUnit, w, "", "")

	return workUnit, mockBaseWorkUnit, mockNetceptor, w, ctrl
}

func TestRemoteWorkUnredactedStatus(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
