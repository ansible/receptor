package controlsvc_test

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/controlsvc/mock_controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/golang/mock/gomock"
)

func TestAddControlFunc(t *testing.T) {

}

func TestRunControlSvc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock_netceptor := mock_controlsvc.NewMockNetceptorForControlsvc(ctrl)
	s := controlsvc.New(false, mock_netceptor)

	// test 1
	mock_netceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("the world blew up"))

	err := s.RunControlSvc(context.Background(), "test", &tls.Config{}, "", os.FileMode(0o600), "", &tls.Config{})

	errorString := "error opening Unix socket: the world blew up"
	if err == nil || err.Error() != errorString {
		t.Errorf("expected error %s, got %s", errorString, err.Error())
	}

	// test 2
	mock_netceptor.EXPECT().ListenAndAdvertise(gomock.Any(), gomock.Any(), gomock.Any()).Return(&netceptor.Listener{}, nil)

	mock_netceptor.EXPECT().GetLogger().Return(logger.NewReceptorLogger("test"))
	err = s.RunControlSvc(context.Background(), "test", &tls.Config{}, "", os.FileMode(0o600), "", &tls.Config{})

	if err != nil {
		t.Errorf("it blew up the second time")
	}

	// test 3
	err = s.RunControlSvc(context.Background(), "", &tls.Config{}, "", os.FileMode(0o600), "", &tls.Config{})

	errorString = "no listeners specified"
	if err == nil || err.Error() != errorString {
		t.Errorf("expected error: %+v, got: %+v", errorString, err.Error())
	}

	// test 4
	invalidAddr := "u81u9u21:"

	mock_netceptor.EXPECT().GetLogger().Return(logger.NewReceptorLogger("test")).AnyTimes()
	err = s.RunControlSvc(context.Background(), "", &tls.Config{}, "", os.FileMode(0o600), invalidAddr, &tls.Config{})

	errorString = fmt.Sprintf("error listening on TCP socket: listen tcp: lookup %s no such host", invalidAddr)

	if err == nil || err.Error() != errorString {
		t.Errorf("expected error: %+v, got: %+v", errorString, err.Error())
	}

	fmt.Println(err)
}
