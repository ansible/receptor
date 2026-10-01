//go:build !no_workceptor
// +build !no_workceptor

package workceptor_test

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"go.uber.org/mock/gomock"
)

func newAdoptControlTest(t *testing.T, tlsErr error) (*workceptor.Workceptor, context.Context, *mock_controlsvc.MockNetceptorForControlCommand, *mock_controlsvc.MockControlFuncOperations, controlsvc.ControlCommandType) {
	t.Helper()
	ctrl := gomock.NewController(t)
	ctx, cancel := context.WithCancel(context.Background())
	mockNC := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
	mockNC.EXPECT().NodeID().Return("adopting-node").AnyTimes()
	mockNC.EXPECT().GetLogger().Return(logger.NewReceptorLogger("")).AnyTimes()
	mockNC.EXPECT().GetClientTLSConfig(gomock.Any(), gomock.Any(), gomock.Any()).Return(&tls.Config{}, tlsErr).AnyTimes()
	mockNC.EXPECT().DialContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("remote unavailable")).AnyTimes()
	w, err := workceptor.New(ctx, mockNC, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mockControlNC := mock_controlsvc.NewMockNetceptorForControlCommand(ctrl)
	mockCFO := mock_controlsvc.NewMockControlFuncOperations(ctrl)
	commandType := workceptor.NewWorkceptorCommandTypeForTest(w)
	t.Cleanup(func() {
		cancel()
		time.Sleep(200 * time.Millisecond)
	})

	return w, ctx, mockControlNC, mockCFO, commandType
}

func TestWorkceptorControlFuncAdopt(t *testing.T) {
	w, ctx, mockControlNC, mockCFO, commandType := newAdoptControlTest(t, nil)
	mockCFO.EXPECT().RemoteAddr().Return(&net.UnixAddr{Net: "unix"}).AnyTimes()
	cmd, err := commandType.InitFromJSON(map[string]interface{}{
		"subcommand": "adopt",
		"node":       "remote-node",
		"unitid":     "remote-unit",
		"tlsclient":  "client",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := cmd.ControlFunc(ctx, mockControlNC, mockCFO)
	if err != nil {
		t.Fatal(err)
	}
	if result["result"] != "Adopted" {
		t.Fatalf("result = %#v, want Adopted", result["result"])
	}
	localUnitID, ok := result["unitid"].(string)
	if !ok || localUnitID == "" {
		t.Fatalf("unitid = %#v, want non-empty string", result["unitid"])
	}
	unit, err := w.UnitStatus(localUnitID)
	if err != nil {
		t.Fatal(err)
	}
	extra, ok := unit.ExtraData.(*workceptor.RemoteExtraData)
	if !ok {
		t.Fatalf("ExtraData = %T, want *RemoteExtraData", unit.ExtraData)
	}
	if extra.RemoteNode != "remote-node" || extra.RemoteUnitID != "remote-unit" || !extra.Adopted || !extra.RemoteStarted || extra.TLSClient != "client" {
		t.Errorf("adopted remote metadata = %#v", extra)
	}
}

func TestWorkceptorControlFuncAdoptRequiresValidSignatureOnTCP(t *testing.T) {
	_, ctx, mockControlNC, mockCFO, commandType := newAdoptControlTest(t, nil)
	mockCFO.EXPECT().RemoteAddr().Return(&net.TCPAddr{}).AnyTimes()
	cmd, err := commandType.InitFromJSON(map[string]interface{}{
		"subcommand": "adopt",
		"node":       "remote-node",
		"unitid":     "remote-unit",
		"signwork":   "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cmd.ControlFunc(ctx, mockControlNC, mockCFO); err == nil || err.Error() != "could not verify signature: signature is empty" {
		t.Fatalf("ControlFunc error = %v, want missing-signature error", err)
	}
}

func TestWorkceptorControlFuncAdoptTLSClientError(t *testing.T) {
	_, ctx, mockControlNC, mockCFO, commandType := newAdoptControlTest(t, errors.New("unknown TLS client"))
	mockCFO.EXPECT().RemoteAddr().Return(&net.UnixAddr{Net: "unix"}).AnyTimes()
	cmd, err := commandType.InitFromJSON(map[string]interface{}{
		"subcommand": "adopt",
		"node":       "remote-node",
		"unitid":     "remote-unit",
		"tlsclient":  "missing-client",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cmd.ControlFunc(ctx, mockControlNC, mockCFO); err == nil || err.Error() != "unknown TLS client" {
		t.Fatalf("ControlFunc error = %v, want TLS client error", err)
	}
}
