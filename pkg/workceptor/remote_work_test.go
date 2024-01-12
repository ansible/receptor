package workceptor_test

import (
	"context"
	"sync"
	"testing"

	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"github.com/golang/mock/gomock"
)

func createRemoteWorkTestSetup(t *testing.T) (workceptor.WorkUnit, *mock_workceptor.MockBaseWorkUnitForWorkUnit, *mock_workceptor.MockNetceptorForWorkceptor, *workceptor.Workceptor) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	mockBaseWorkUnit := mock_workceptor.NewMockBaseWorkUnitForWorkUnit(ctrl)
	mockNetceptor := mock_workceptor.NewMockNetceptorForWorkceptor(ctrl)
	mockNetceptor.EXPECT().NodeID().Return("NodeID")
	mockNetceptor.EXPECT().GetLogger()

	w, err := workceptor.New(ctx, mockNetceptor, "/tmp")
	if err != nil {
		t.Errorf("Error while creating Workceptor: %v", err)
	}

	mockBaseWorkUnit.EXPECT().Init(w, "", "", workceptor.FileSystem{}, nil)
	mockBaseWorkUnit.EXPECT().SetStatusExtraData(gomock.Any())
	workUnit := workceptor.NewRemoteWorker(mockBaseWorkUnit, w, "", "")

	return workUnit, mockBaseWorkUnit, mockNetceptor, w
}

func TestRemoteWorkUnredactedStatus(t *testing.T) {
	t.Parallel()
	wu, mockBaseWorkUnit, _, _ := createRemoteWorkTestSetup(t)
	restartTestCases := []struct {
		name string
	}{
		{name: "test1"},
		{name: "test2"},
		{name: "test3"},
	}

	for _, testCase := range restartTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			statusLock := &sync.RWMutex{}
			mockBaseWorkUnit.EXPECT().GetStatusLock().Return(statusLock).Times(2)
			mockBaseWorkUnit.EXPECT().GetStatusWithoutExtraData().Return(&workceptor.StatusFileData{})
			mockBaseWorkUnit.EXPECT().GetStatusCopy().Return(workceptor.StatusFileData{
				ExtraData: &workceptor.RemoteExtraData{},
			})
			wu.UnredactedStatus()
		})
	}
}
