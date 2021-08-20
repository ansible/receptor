package netceptor

import (
	"testing"
)

func TestFirewallRules(t *testing.T) {
	// Rule #1
	rule, err := ParseFirewallRule("all:accept")
	if err != nil {
		t.Fatal(err)
	}
	if rule(&MessageData{}) != FirewallResultAccept {
		t.Fatal("rule #1 did not return Accept")
	}

	// Rule #2
	rule, err = ParseFirewallRule("FromNode=foo, ToNode=bar, ToService=control: drop")
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
	rule, err = ParseFirewallRule("fromnode = /a.*b/ : reject")
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
	rule, err = ParseFirewallRule("TONODE = /(?i)a.*b/ : reject")
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
