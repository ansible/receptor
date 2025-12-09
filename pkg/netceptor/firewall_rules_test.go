package netceptor

import (
	"strings"
	"testing"
)

type firewallRuleTestCase struct {
	desc      string
	ruleIndex int
	data      *MessageData
	want      FirewallResult
}

type compareTestCase struct {
	desc      string
	data      *MessageData
	wantMatch bool
}

func TestFirewallRules(t *testing.T) {
	var frd FirewallRuleData

	// Rule #1
	frd = FirewallRuleData{}
	frd["action"] = "accept"
	rule, err := frd.ParseFirewallRule()
	if err != nil {
		t.Fatal(err)
	}
	if rule(&MessageData{}) != FirewallResultAccept {
		t.Fatal("rule #1 did not return Accept")
	}

	// // Rule #2
	frd = FirewallRuleData{}
	frd["Action"] = "drop"
	frd["FromNode"] = "foo"
	frd["ToNode"] = "bar"
	frd["ToService"] = "control"
	rule, err = frd.ParseFirewallRule()
	if err != nil {
		t.Fatal(err)
	}
	if rule(&MessageData{}) != FirewallResultContinue {
		t.Fatal("rule #2 did not return Continue")
	}
	if rule(&MessageData{
		FromNode:  "foo",
		ToNode:    "bar",
		ToService: "control",
	}) != FirewallResultDrop {
		t.Fatal("rule #2 did not return Drop")
	}

	// Rule #3
	frd = FirewallRuleData{}
	frd["fromnode"] = "/a.*b/"
	frd["action"] = "reject"
	rule, err = frd.ParseFirewallRule()
	if err != nil {
		t.Fatal(err)
	}
	if rule(&MessageData{}) != FirewallResultContinue {
		t.Fatal("rule #3 did not return Continue")
	}
	if rule(&MessageData{
		FromNode: "appleb",
	}) != FirewallResultReject {
		t.Fatal("rule #3 did not return Reject")
	}
	if rule(&MessageData{
		FromNode: "Appleb",
	}) != FirewallResultContinue {
		t.Fatal("rule #3 did not return Continue")
	}

	// Rule #4
	frd = FirewallRuleData{}
	frd["TONODE"] = "/(?i)a.*b/"
	frd["ACTION"] = "reject"
	rule, err = frd.ParseFirewallRule()
	if err != nil {
		t.Fatal(err)
	}
	if rule(&MessageData{}) != FirewallResultContinue {
		t.Fatal("rule #4 did not return Continue")
	}
	if rule(&MessageData{
		ToNode: "appleb",
	}) != FirewallResultReject {
		t.Fatal("rule #4 did not return Reject")
	}
	if rule(&MessageData{
		ToNode: "Appleb",
	}) != FirewallResultReject {
		t.Fatal("rule #4 did not return Reject")
	}
}

func TestParseFirewallRules(t *testing.T) {
	tests := []struct {
		name            string
		rules           []FirewallRuleData
		wantErr         bool
		wantErrContains string
		wantCount       int
		testCases       []firewallRuleTestCase
	}{
		// Test with empty rules slice
		{
			name:      "empty rules slice",
			rules:     []FirewallRuleData{},
			wantErr:   false,
			wantCount: 0,
		},
		// Test with single valid rule
		{
			name: "single valid rule",
			rules: []FirewallRuleData{
				{
					"action":   "accept",
					"fromnode": "node1",
				},
			},
			wantErr:   false,
			wantCount: 1,
			testCases: []firewallRuleTestCase{
				{
					desc:      "rule 0 accepts matching node1",
					ruleIndex: 0,
					data:      &MessageData{FromNode: "node1"},
					want:      FirewallResultAccept,
				},
				{
					desc:      "rule 0 continues for non-matching node2",
					ruleIndex: 0,
					data:      &MessageData{FromNode: "node2"},
					want:      FirewallResultContinue,
				},
			},
		},
		// Test with multiple valid rules
		{
			name: "multiple valid rules",
			rules: []FirewallRuleData{
				{
					"action":   "accept",
					"fromnode": "node1",
				},
				{
					"action":   "reject",
					"fromnode": "node2",
				},
			},
			wantErr:   false,
			wantCount: 2,
			testCases: []firewallRuleTestCase{
				{
					desc:      "rule 0 accepts matching node1",
					ruleIndex: 0,
					data:      &MessageData{FromNode: "node1"},
					want:      FirewallResultAccept,
				},
				{
					desc:      "rule 0 continues for non-matching node2",
					ruleIndex: 0,
					data:      &MessageData{FromNode: "node2"},
					want:      FirewallResultContinue,
				},
				{
					desc:      "rule 1 rejects matching node2",
					ruleIndex: 1,
					data:      &MessageData{FromNode: "node2"},
					want:      FirewallResultReject,
				},
				{
					desc:      "rule 1 continues for non-matching node1",
					ruleIndex: 1,
					data:      &MessageData{FromNode: "node1"},
					want:      FirewallResultContinue,
				},
			},
		},

		// Test that error message includes the rule index number
		{
			name: "error message includes rule index",
			rules: []FirewallRuleData{
				{
					"action":   "accept",
					"fromnode": "node1",
				},
				{
					"action":     "reject",
					"invalidkey": "value",
				},
				{
					"action":   "drop",
					"fromnode": "node3",
				},
			},
			wantErr:         true,
			wantErrContains: "error in rule 1",
			wantCount:       0,
		},
		// Test with invalid rule data
		{
			name: "empty value",
			rules: []FirewallRuleData{
				{
					"action": "",
				},
			},
			wantErr:         true,
			wantCount:       0,
			wantErrContains: "unknown action",
		},
		{
			name: "error message for invalid key",
			rules: []FirewallRuleData{
				{
					"invalidkey": "value",
				},
			},
			wantErr:         true,
			wantErrContains: "invalid firewall rule. unknown key: invalidkey",
			wantCount:       0,
		},
		{
			name: "empty map with no action",
			rules: []FirewallRuleData{
				{},
			},
			wantErr:         true,
			wantErrContains: "unknown action",
			wantCount:       0,
		},
		{
			name: "nil map",
			rules: []FirewallRuleData{
				nil,
			},
			wantErr:         true,
			wantErrContains: "unknown action",
			wantCount:       0,
		},
		{
			name: "non-string value",
			rules: []FirewallRuleData{
				{
					493: 123,
				},
			},
			wantErr:         true,
			wantErrContains: "invalid firewall rule. <int Value> must be a string",
			wantCount:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := ParseFirewallRules(tt.rules)
			// Check error expectations
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseFirewallRules() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check error message
			if tt.wantErrContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("ParseFirewallRules() error = %v, want error containing %q", err, tt.wantErrContains)
				}
			}

			// Check number of rules
			if len(rules) != tt.wantCount {
				t.Errorf("ParseFirewallRules() returned %d rules, want %d", len(rules), tt.wantCount)
			}

			// Run test cases
			for _, tc := range tt.testCases {
				got := rules[tc.ruleIndex](tc.data)
				if got != tc.want {
					t.Errorf("%s: got %v, want %v", tc.desc, got, tc.want)
				}
			}
		})
	}
}

func TestRegexCompare(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		value     string
		wantErr   bool
		testCases []compareTestCase
	}{
		// Test with invalid regex patterns
		{
			name:    "unclosed bracket",
			field:   "fromnode",
			value:   "/[unclosed/",
			wantErr: true,
		},
		{
			name:    "invalid escape",
			field:   "fromnode",
			value:   "/node\\k/",
			wantErr: true,
		},
		{
			name:    "bad repetition range",
			field:   "fromnode",
			value:   "/node{5,2}/",
			wantErr: true,
		},
		{
			name:    "not enclosed in slashes",
			field:   "fromnode",
			value:   "node.*",
			wantErr: true,
		},
		// Test with empty regex pattern string
		{
			name:    "matches only empty strings",
			field:   "toservice",
			value:   "//",
			wantErr: false,
			testCases: []compareTestCase{
				{
					desc:      "matches empty string",
					data:      &MessageData{ToService: ""},
					wantMatch: true,
				},
				{
					desc:      "does not match non-empty string",
					data:      &MessageData{ToService: "343.236"},
					wantMatch: false,
				},
			},
		},
		// Test with complex regex patterns
		{
			name:    "exactly 3 digits",
			field:   "fromnode",
			value:   "/node-[0-9]{3}/",
			wantErr: false,
			testCases: []compareTestCase{
				{
					desc:      "matches node-123",
					data:      &MessageData{FromNode: "node-123"},
					wantMatch: true,
				},
				{
					desc:      "does not match node-12",
					data:      &MessageData{FromNode: "node-12"},
					wantMatch: false,
				},
				{
					desc:      "does not match node-1234",
					data:      &MessageData{FromNode: "node-1234"},
					wantMatch: false,
				},
			},
		},
		// Test regex matching with special characters
		{
			name:    "escaped dots for IP address",
			field:   "fromnode",
			value:   "/192\\.168\\.1\\.1/",
			wantErr: false,
			testCases: []compareTestCase{
				{
					desc:      "matches exact IP",
					data:      &MessageData{FromNode: "192.168.1.1"},
					wantMatch: true,
				},
				{
					desc:      "does not match with other chars",
					data:      &MessageData{FromNode: "192X168Y1Z1"},
					wantMatch: false,
				},
			},
		},
		{
			name:    "escaped parentheses",
			field:   "fromservice",
			value:   "/api-v2\\.0\\(prod\\)/",
			wantErr: false,
			testCases: []compareTestCase{
				{
					desc:      "matches api-v2.0(prod)",
					data:      &MessageData{FromService: "api-v2.0(prod)"},
					wantMatch: true,
				},
				{
					desc:      "does not match without parentheses",
					data:      &MessageData{FromService: "api-v2.0"},
					wantMatch: false,
				},
			},
		},
		{
			name:    "escaped brackets",
			field:   "tonode",
			value:   "/server\\[prod\\]/",
			wantErr: false,
			testCases: []compareTestCase{
				{
					desc:      "matches server[prod]",
					data:      &MessageData{ToNode: "server[prod]"},
					wantMatch: true,
				},
				{
					desc:      "does not match server(prod)",
					data:      &MessageData{ToNode: "server(prod)"},
					wantMatch: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp, err := regexCompare(tt.field, tt.value)
			// Check error expectations
			if (err != nil) != tt.wantErr {
				t.Fatalf("regexCompare() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Run test cases
			if !tt.wantErr && comp != nil {
				for _, tc := range tt.testCases {
					got := comp(tc.data)
					if got != tc.wantMatch {
						t.Errorf("%s: comp() = %v, want %v", tc.desc, got, tc.wantMatch)
					}
				}
			}
		})
	}
}

func TestStringCompare(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		value     string
		wantErr   bool
		testCases []compareTestCase
	}{
		// Test with empty value string
		{
			name:    "empty value string",
			field:   "fromnode",
			value:   "",
			wantErr: false,
			testCases: []compareTestCase{
				{
					desc:      "matches empty FromNode",
					data:      &MessageData{FromNode: ""},
					wantMatch: true,
				},
				{
					desc:      "does not match empty FromNode",
					data:      &MessageData{FromNode: "node-1"},
					wantMatch: false,
				},
			},
		},
		// Test with empty field string
		{
			name:    "empty field string",
			field:   "",
			value:   "tonode",
			wantErr: true,
		},
		// Test with empty field and empty value string
		{
			name:    "empty field and value string",
			field:   "",
			value:   "",
			wantErr: true,
		},
		// Test with special characters in field string
		{
			name:    "special characters in field string",
			field:   "toservice!",
			value:   "production",
			wantErr: true,
		},
		// Test with special characters in value string
		{
			name:    "special characters in value string",
			field:   "tonode",
			value:   "node-123&",
			wantErr: false,
			testCases: []compareTestCase{
				{
					desc:      "matches value with special characters",
					data:      &MessageData{ToNode: "node-123&"},
					wantMatch: true,
				},
				{
					desc:      "does not match value with special characters",
					data:      &MessageData{ToNode: "node123"},
					wantMatch: false,
				},
			},
		},
		// Test mixed case characters
		{
			name:    "mixed cases in field string",
			field:   "ToService",
			value:   "Prod-123",
			wantErr: false,
			testCases: []compareTestCase{
				{
					desc:      "matches field with mixed case characters",
					data:      &MessageData{ToService: "Prod-123"},
					wantMatch: true,
				},
				{
					desc:      "does not match value with lower-case characters",
					data:      &MessageData{ToService: "prod-123"},
					wantMatch: false,
				},
			},
		},
		// Test uppercase characters
		{
			name:    "upper case in field string",
			field:   "FROMSERVICE",
			value:   "dev.83",
			wantErr: false,
			testCases: []compareTestCase{
				{
					desc:      "matches field with upper-case characters",
					data:      &MessageData{FromService: "dev.83"},
					wantMatch: true,
				},
			},
		},
		// Test unknown field name
		{
			name:    "unknown field name",
			field:   "unknown",
			value:   "node-85",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp, err := stringCompare(tt.field, tt.value)
			// Check error expectations
			if (err != nil) != tt.wantErr {
				t.Fatalf("stringCompare() error = %v, want %v", err, tt.wantErr)
			}

			// Run test cases
			if !tt.wantErr && comp != nil {
				for _, tc := range tt.testCases {
					got := comp(tc.data)
					if got != tc.wantMatch {
						t.Errorf("%s: comp() = %v, want %v", tc.desc, got, tc.wantMatch)
					}
				}
			}
		})
	}
}
