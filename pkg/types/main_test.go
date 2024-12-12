package types

import "testing"

func TestMainInitNodeID(t *testing.T) {
	mainInitNodeIDTestCases := []struct {
		name        string
		nodeID      string
		expectedErr string
	}{
		{
			name:        "successful, no error",
			nodeID:      "t.e-s_t@1:234",
			expectedErr: "",
		},
		{
			name:        "failed, charactered not allowed",
			nodeID:      "test!#&123",
			expectedErr: "node id can only contain a-z, A-Z, 0-9 or special characters . - _ @ : but received: test!#&123",
		},
	}

	for _, testCase := range mainInitNodeIDTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := NodeCfg{
				ID: testCase.nodeID,
			}
			err := cfg.Init()
			if err == nil && testCase.expectedErr != "" {
				t.Errorf("exected error but got no error")
			} else if err != nil && err.Error() != testCase.expectedErr {
				t.Errorf("expected error to be %s, but got: %s", testCase.expectedErr, err.Error())
			}
			t.Cleanup(func() {
				cfg = NodeCfg{}
			})
		})
	}
}
