package netceptor

import (
	"fmt"
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer/stateful"
	"regexp"
	"strings"
)

/*

This file provides a parser for firewall rules provided by the CLI.  It takes strings
and returns rule functions suitable for passing to AddFirewallRules().

The firewall language is as follows:

RULE[, RULE, ...]: ACTION

Each RULE is either the word "all", or an expression of the form field=value or field=/regex/, where field
can be FromNode, FromService, ToNode or ToService.  Regular expressions must match the whole string, not
a substring.  A special RULE of "all" matches everything.  ACTION can be one of Accept, Reject or Drop.
All tokens are case insensitive and all whitespace is optional; field values contents are case sensitive.

Example rules:

all:accept
  Accepts everything

FromNode=foo, ToNode=bar, ToService=control: drop
  Silently drops messages from foo to bar's control service
  Note that "foo", "bar" and "control" are case sensitive

fromnode = /a.*b/ : reject
  Rejects messages originating from any node whose node ID starts with a and ends with b

TONODE = /(?i)a.*b/ : reject
  Rejects messages destined for any node whose node ID starts with a or A and ends with b or B

*/

// ParseFirewallRule takes a single string describing a firewall rule, and returns a FirewallRule function
func ParseFirewallRule(rule string) (FirewallRule, error) {
	parsedRule := &ruleString{}
	err := ruleParser.ParseString("rule", rule, parsedRule)
	if err != nil {
		return nil, err
	}
	comps := make([]compareFunc, 0)
	for _, pr := range parsedRule.RuleSpec.Rules {
		if pr.Value.Value != "" {
			comp, err := stringCompare(pr.Field, pr.Value.Value)
			if err != nil {
				return nil, err
			}
			comps = append(comps, comp)
		} else if pr.Value.Regex != "" {
			comp, err := regexCompare(pr.Field, pr.Value.Regex)
			if err != nil {
				return nil, err
			}
			comps = append(comps, comp)
		} else {
			return nil, fmt.Errorf("no value or regex provided for field %s", pr.Field)
		}
	}
	fwr, err := firewallRule(comps, parsedRule.Action)
	if err != nil {
		return nil, err
	}
	return fwr, nil
}

// ParseFirewallRules takes a slice of string describing firewall rules, and returns a slice of FirewallRule functions
func ParseFirewallRules(rules []string) ([]FirewallRule, error) {
	results := make([]FirewallRule, 0)
	for i, rule := range rules {
		result, err := ParseFirewallRule(rule)
		if err != nil {
			return nil, fmt.Errorf("error in rule %d: %s", i, err)
		}
		results = append(results, result)
	}
	return results, nil
}

type compareFunc func(md *MessageData) bool

func firewallRule(comparers []compareFunc, action string) (FirewallRule, error) {
	var result FirewallResult
	switch strings.ToLower(action) {
	case "accept":
		result = FirewallResultAccept
	case "reject":
		result = FirewallResultReject
	case "drop":
		result = FirewallResultDrop
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
	if len(comparers) == 0 {
		return func(md *MessageData) FirewallResult {
			return result
		}, nil
	}
	return func(md *MessageData) FirewallResult {
		matched := true
		for _, comp := range comparers {
			matched = matched && comp(md)
		}
		if matched {
			return result
		}
		return FirewallResultContinue
	}, nil
}

func stringCompare(field string, value string) (compareFunc, error) {
	switch strings.ToLower(field) {
	case "fromnode":
		return func(md *MessageData) bool {
			return md.FromNode == value
		}, nil
	case "fromservice":
		return func(md *MessageData) bool {
			return md.FromService == value
		}, nil
	case "tonode":
		return func(md *MessageData) bool {
			return md.ToNode == value
		}, nil
	case "toservice":
		return func(md *MessageData) bool {
			return md.ToService == value
		}, nil
	}
	return nil, fmt.Errorf("unknown field: %s", field)
}

func regexCompare(field string, value string) (compareFunc, error) {
	if value[0] != '/' || value[len(value)-1] != '/' {
		return nil, fmt.Errorf("regex not enclosed in //")
	}
	value = fmt.Sprintf("^%s$", value[1:len(value)-1])
	re, err := regexp.Compile(value)
	if err != nil {
		return nil, fmt.Errorf("regex failed to compile: %s", value)
	}
	switch strings.ToLower(field) {
	case "fromnode":
		return func(md *MessageData) bool {
			return re.MatchString(md.FromNode)
		}, nil
	case "fromservice":
		return func(md *MessageData) bool {
			return re.MatchString(md.FromService)
		}, nil
	case "tonode":
		return func(md *MessageData) bool {
			return re.MatchString(md.ToNode)
		}, nil
	case "toservice":
		return func(md *MessageData) bool {
			return re.MatchString(md.ToService)
		}, nil
	}
	return nil, fmt.Errorf("unknown field: %s", field)
}

//goland:noinspection GoVetStructTag
type ruleString struct {
	RuleSpec *ruleSpec `@@`
	Action   string    `":" @Ident`
}

//goland:noinspection GoVetStructTag
type ruleSpec struct {
	All   string  `@"all"`
	Rules []*rule `| @@ ( "," @@ )*`
}

//goland:noinspection GoVetStructTag
type rule struct {
	Field string     `@Ident`
	Value *ruleValue `@@`
}

//goland:noinspection GoVetStructTag
type ruleValue struct {
	Value string `"=" (@Ident|@Text)`
	Regex string `| "=" @Regex`
}

var (
	ruleLexer = stateful.MustSimple([]stateful.Rule{
		{"Ident", `\w+`, nil},
		{"Regex", `\/((?:[^/\\]|\\.)*)\/`, nil},
		{"Text", `[^/,:= \t\r\n][^,:= \t\r\n]*`, nil},
		{"Whitespace", `\s+`, nil},
		{"Punctuation", `[-[~!@#$%^&*()+_={}\|:;"'<,>.?/]|]`, nil},
	})
	ruleParser = participle.MustBuild(&ruleString{},
		participle.Lexer(ruleLexer),
		participle.Elide("Whitespace"),
		participle.UseLookahead(2))
)
