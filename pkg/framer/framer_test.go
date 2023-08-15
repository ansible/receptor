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
		result := f.SendData(testCase.inputBuffer)

		if !bytes.Equal(testCase.expectedBuffer, result) {
			t.Errorf("%s - expected: %+v, received: %+v", testCase.name, testCase.expectedBuffer, result)
		}
	}
}
