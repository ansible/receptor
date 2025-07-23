package framer_test

import (
	"bytes"
	"testing"

	"github.com/ansible/receptor/pkg/framer"
)

func TestSendData(t *testing.T) {
	f := framer.New()

	smallBuffer := []byte{1, 2, 3, 4, 5, 6}
	largeBuffer := []byte{}
	for i := 1; i <= 271; i++ {
		largeBuffer = append(largeBuffer, byte(i))
	}

	framedBufferTestCases := []struct {
		name           string
		inputBuffer    []byte
		expectedBuffer []byte
	}{
		{
			name:           "small buffer",
			inputBuffer:    smallBuffer,
			expectedBuffer: append([]byte{6, 0}, smallBuffer...),
		},
		{
			name:           "large buffer",
			inputBuffer:    largeBuffer,
			expectedBuffer: append([]byte{15, 1}, largeBuffer...),
		},
	}

	for _, testCase := range framedBufferTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := f.SendData(testCase.inputBuffer)

			if !bytes.Equal(testCase.expectedBuffer, result) {
				t.Errorf("%s - expected: %+v, received: %+v", testCase.name, testCase.expectedBuffer, result)
			}
		})
	}
}

func TestMessageReady(t *testing.T) {
	buffer := []byte{1, 2, 3}

	messageReadyTestCases := []struct {
		name           string
		inputBuffer    []byte
		expectedResult bool
	}{
		{
			name:           "message ready",
			inputBuffer:    append([]byte{3, 0}, buffer...),
			expectedResult: true,
		},
		{
			name:           "message not ready",
			inputBuffer:    buffer,
			expectedResult: false,
		},
	}

	for _, testCase := range messageReadyTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := framer.New()
			f.RecvData(testCase.inputBuffer)
			receivedResult := f.MessageReady()

			if testCase.expectedResult != receivedResult {
				t.Errorf("%s - expected: %+v, received: %+v", testCase.name, testCase.expectedResult, receivedResult)
			}
		})
	}
}

func TestGetMessage(t *testing.T) {
	buffer := []byte{1, 2, 3}
	framedBuffer := append([]byte{3, 0}, buffer...)

	getMessageTestCases := []struct {
		name          string
		inputBuffer   []byte
		expectedError bool
	}{
		{
			name:          "message read",
			inputBuffer:   framedBuffer,
			expectedError: false,
		},
		{
			name:          "message not read with sending non framed buffer",
			inputBuffer:   buffer,
			expectedError: true,
		},
		{
			name:          "message not read with sending insufficient buffer length",
			inputBuffer:   []byte{1},
			expectedError: true,
		},
	}

	for _, testCase := range getMessageTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := framer.New()
			f.RecvData(testCase.inputBuffer)

			receivedResult, err := f.GetMessage()

			if testCase.expectedError && err == nil {
				t.Error(testCase.name)
			}
			if !testCase.expectedError && !bytes.Equal(buffer, receivedResult) {
				t.Errorf("expected: %+v, received: %+v", buffer, receivedResult)
			}
		})
	}
}
