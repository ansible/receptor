package netceptor

import (
	"testing"
)

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
