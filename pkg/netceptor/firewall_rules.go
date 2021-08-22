package netceptor

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

type FirewallRuleData map[interface{}]interface{}

type FirewallRule struct {
	Action      string
	FromNode    string
	ToNode      string
	FromService string
	ToService   string
}

func buildComp(field string, pattern string) CompareFunc {
	if pattern == "" {
		return nil
	}
	var comp CompareFunc
	if strings.HasPrefix(pattern, "/") {
		comp, _ = regexCompare(field, pattern)
	} else {
		comp, _ = stringCompare(field, pattern)
	}

	return comp
}

func (fr FirewallRule) BuildComps() []CompareFunc {
	var comps []CompareFunc
	fnc := buildComp("fromnode", fr.FromNode)
	if fnc != nil {
		comps = append(comps, fnc)
	}

	tnc := buildComp("tonode", fr.ToNode)
	if tnc != nil {
		comps = append(comps, tnc)
	}

	fsc := buildComp("fromservice", fr.FromService)
	if fsc != nil {
		comps = append(comps, fsc)
	}

	tsc := buildComp("toservice", fr.ToService)
	if tsc != nil {
		comps = append(comps, tsc)
	}

	return comps
}

// ParseFirewallRule takes a single string describing a firewall rule, and returns a FirewallRuleFunc function.
func (frd FirewallRuleData) ParseFirewallRule() (FirewallRuleFunc, error) {
	rv := reflect.ValueOf(frd)
	if rv.Kind() != reflect.Map {
		return nil, fmt.Errorf("invalid firewall rule. see documentation for syntax")
	}

	fr := FirewallRule{}
	for _, key := range rv.MapKeys() {
		mkv := rv.MapIndex(key)
		key := key.Elem().String()

		switch mkv.Interface().(type) {
		case string:
			// expected
		default:
			return nil, fmt.Errorf("invalid firewall rule. %s must be a string", key)
		}

		val := mkv.Elem().String()

		switch strings.ToLower(key) {
		case "action":
			fr.Action = val
		case "fromnode":
			fr.FromNode = val
		case "tonode":
			fr.ToNode = val
		case "fromservice":
			fr.FromService = val
		case "toservice":
			fr.ToService = val
		default:
			return nil, fmt.Errorf("invalid filewall rule. unknown key: %s", key)
		}
	}

	comps := fr.BuildComps()
	fwr, err := firewallRule(comps, fr.Action)
	if err != nil {
		return nil, err
	}

	return fwr, nil
}

// ParseFirewallRules takes a slice of string describing firewall rules, and returns a slice of FirewallRuleFunc functions.
func ParseFirewallRules(rules []FirewallRuleData) ([]FirewallRuleFunc, error) {
	results := make([]FirewallRuleFunc, 0)
	for i, rule := range rules {
		result, err := rule.ParseFirewallRule()
		if err != nil {
			return nil, fmt.Errorf("error in rule %d: %s", i, err)
		}
		results = append(results, result)
	}

	return results, nil
}

type CompareFunc func(md *MessageData) bool

func firewallRule(comparers []CompareFunc, action string) (FirewallRuleFunc, error) {
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

func stringCompare(field string, value string) (CompareFunc, error) {
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

func regexCompare(field string, value string) (CompareFunc, error) {
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
