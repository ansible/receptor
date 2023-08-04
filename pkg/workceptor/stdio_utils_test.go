package workceptor_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/ansible/receptor/pkg/workceptor/mock_workceptor"
	"github.com/golang/mock/gomock"
)

// checkErrorAndNum checks common return types against expected values.
func checkErrorAndNum(err error, expectedErr string, num int, expectedNum int, t *testing.T) {
	if expectedErr == "" && err != nil {
		t.Errorf("Expected no error, got: %v", err)
	} else if expectedErr != "" && (err == nil || err.Error() != expectedErr) {
		t.Errorf("Expected error: %s, got: %v", expectedErr, err)
	}
	if num != expectedNum {
		t.Errorf("Expected num to be %d, got: %d", expectedNum, num)
	}
}

func TestWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockfilesystemer := mock_workceptor.NewMockFileSystemer(ctrl)
	mockfilewc := mock_workceptor.NewMockFileWriteCloser(ctrl)

	mockfilesystemer.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(&os.File{}, nil)
	wc, err := workceptor.NewStdoutWriter(mockfilesystemer, "")
	if err != nil {
		t.Errorf("Error while creating std writer: %v", err)
	}
	wc.SetWriter(mockfilewc)

	writeTestCases := []struct {
		name        string
		returnNum   int
		returnErr   error
		expectedNum int
		expectedErr string
	}{
		{"Write OK", 0, nil, 0, ""},
		{"Write OK, correct num returned", 1, nil, 1, ""},
		{"Write Error", 0, errors.New("Write error"), 0, "Write error"},
	}

	for _, testCase := range writeTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockfilewc.EXPECT().Write(gomock.Any()).Return(testCase.returnNum, testCase.returnErr)
			num, err := wc.Write([]byte(gomock.Any().String()))
			checkErrorAndNum(err, testCase.expectedErr, num, testCase.expectedNum, t)
		})
	}
}

func TestWriteSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockfilesystemer := mock_workceptor.NewMockFileSystemer(ctrl)

	mockfilesystemer.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(&os.File{}, nil)
	wc, err := workceptor.NewStdoutWriter(mockfilesystemer, "")
	if err != nil {
		t.Errorf("Error while creating std writer: %v", err)
	}

	sizeTestCases := []struct {
		name         string
		expectedSize int64
	}{
		{name: "Size return OK", expectedSize: 0},
	}

	for _, testCase := range sizeTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			num := wc.Size()
			if num != testCase.expectedSize {
				t.Errorf("Expected size to be %d, got: %d", testCase.expectedSize, num)
			}
		})
	}
}

type Info struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func NewInfo(name string, size int64, mode os.FileMode, modTime time.Time) *Info {
	return &Info{
		name:    name,
		size:    size,
		mode:    mode,
		modTime: modTime,
	}
}

func (i *Info) Name() string {
	return i.name
}

func (i *Info) Size() int64 {
	return i.size
}

func (i *Info) IsDir() bool {
	return i.mode.IsDir()
}

func (i *Info) Mode() os.FileMode {
	return i.mode
}

func (i *Info) ModTime() time.Time {
	return i.modTime
}

func (i *Info) Sys() interface{} {
	return nil
}

const stdinError = "Error creating stdinReader"

func TestRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockfilesystemer := mock_workceptor.NewMockFileSystemer(ctrl)
	statObj := NewInfo("test", 1, 0, time.Now())

	mockfilesystemer.EXPECT().Stat(gomock.Any()).Return(statObj, nil)
	mockfilesystemer.EXPECT().Open(gomock.Any()).Return(&os.File{}, nil)

	wc, err := workceptor.NewStdinReader(mockfilesystemer, "")
	if err != nil {
		t.Errorf(stdinError)
	}

	mockReadClose := mock_workceptor.NewMockFileReadCloser(ctrl)
	wc.SetReader(mockReadClose)

	readTestCases := []struct {
		name        string
		returnNum   int
		returnErr   error
		expectedNum int
		expectedErr string
	}{
		{"Read ok", 1, nil, 1, ""},
		{"Read Error", 1, errors.New("Read error"), 1, "Read error"},
	}

	for _, testCase := range readTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockReadClose.EXPECT().Read(gomock.Any()).Return(testCase.returnNum, testCase.returnErr)
			num, err := wc.Read([]byte(gomock.Any().String()))
			checkErrorAndNum(err, testCase.expectedErr, num, testCase.expectedNum, t)
		})
	}
}

func TestDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockfilesystemer := mock_workceptor.NewMockFileSystemer(ctrl)
	statObj := NewInfo("test", 1, 0, time.Now())

	mockfilesystemer.EXPECT().Stat(gomock.Any()).Return(statObj, nil)
	mockfilesystemer.EXPECT().Open(gomock.Any()).Return(&os.File{}, nil)

	wc, err := workceptor.NewStdinReader(mockfilesystemer, "")
	if err != nil {
		t.Errorf(stdinError)
	}

	channel := wc.Done()
	if channel == nil {
		t.Errorf("Done chan is set to nil")
	}
}

func TestError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockfilesystemer := mock_workceptor.NewMockFileSystemer(ctrl)
	statObj := NewInfo("test", 1, 0, time.Now())

	mockfilesystemer.EXPECT().Stat(gomock.Any()).Return(statObj, nil)
	mockfilesystemer.EXPECT().Open(gomock.Any()).Return(&os.File{}, nil)

	wc, err := workceptor.NewStdinReader(mockfilesystemer, "")
	if err != nil {
		t.Errorf(stdinError)
	}

	err = wc.Error()
	if err != nil {
		t.Errorf("Unexpected error returned from stdreader")
	}
}
