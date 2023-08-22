package randstr_test

import (
	"strings"
	"testing"

	"github.com/ansible/receptor/pkg/randstr"
)

func TestRandStrLength(t *testing.T) {
	randStringTestCases := []struct {
		name           string
		inputLength    int
		expectedLength int
	}{
		{
			name:           "length of 100",
			inputLength:    100,
			expectedLength: 100,
		},
		{
			name:           "length of 0",
			inputLength:    0,
			expectedLength: 0,
		},
		{
			name:           "length of -1",
			inputLength:    -1,
			expectedLength: 0,
		},
	}

	for _, testCase := range randStringTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			randomStr := randstr.RandomString(testCase.inputLength)

			if len(randomStr) != testCase.expectedLength {
				t.Errorf("%s - expected: %+v, received: %+v", testCase.name, testCase.expectedLength, len(randomStr))
			}
		})
	}
}

func TestRandStrHasDifferentOutputThanCharset(t *testing.T) {
	charset := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	randomStr := randstr.RandomString(len(charset))

	if randomStr == charset {
		t.Errorf("output should be different than charset. charset: %+v, received: %+v", charset, randomStr)
	}
}

func TestRandStrHasNoContinuousSubStringOfCharset(t *testing.T) {
	randomStr := randstr.RandomString(10)
	charset := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	charsetIndex := strings.Index(charset, string(randomStr[0]))
	for index, char := range randomStr {
		if index == 0 {
			continue
		}
		currentCharsetIndex := strings.Index(charset, string(char))
		if charsetIndex+1 != currentCharsetIndex {
			break
		}
		if index+1 == len(randomStr) {
			t.Error("rand str is continuous")
		}
		charsetIndex = currentCharsetIndex
	}
}
