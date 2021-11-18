package controlsvc

import (
	"testing"
)

func TestReload(t *testing.T) {
	type yamltest struct {
		filename    string
		parseError  bool
		reloadError bool
		expectBool  []bool // backendModified, loglevelModified, loglevelPresent
	}

	scenarios := []yamltest{
		{filename: "reload_test_yml/init.yml", parseError: false, reloadError: false, expectBool: []bool{false, false, true}},
		{filename: "reload_test_yml/add_cfg.yml", parseError: false, reloadError: true, expectBool: []bool{false, false, true}},
		{filename: "reload_test_yml/drop_cfg.yml", parseError: false, reloadError: true, expectBool: []bool{false, false, true}},
		{filename: "reload_test_yml/modify_cfg.yml", parseError: false, reloadError: true, expectBool: []bool{false, false, true}},
		{filename: "reload_test_yml/syntax_error.yml", parseError: true, reloadError: false, expectBool: []bool{false, false, true}},
		{filename: "reload_test_yml/successful_reload.yml", parseError: false, reloadError: false, expectBool: []bool{true, true, true}},
		{filename: "reload_test_yml/remove_log.yml", parseError: false, reloadError: false, expectBool: []bool{false, true, false}},
	}

	for _, s := range scenarios {
		cfgPrevious = make(map[string]struct{})
		cfgNext = make(map[string]struct{})
		resetAfterReload()

		t.Log(s.filename)
		err := parseConfig("reload_test_yml/init.yml", cfgPrevious)
		if err != nil {
			t.Fatal("could not parse a good-syntax file:", err.Error())
		}
		if len(cfgPrevious) != 6 {
			t.Fatal("incorrect cfgPrevious length")
		}

		err = parseConfig(s.filename, cfgNext)
		if err != nil && !s.parseError {
			t.Fatal("could not parse the modified file:", err.Error())
		}
		if err == nil && s.parseError {
			t.Fatal("expected error when parsing file")
		}
		if s.parseError {
			continue
		}

		err = checkReload()
		if err != nil && !s.reloadError {
			t.Fatal("did not expect error:", err.Error())
		}
		if err == nil && s.reloadError {
			t.Fatal("expected error when reloading file")
		}
		if s.reloadError {
			continue
		}

		for i, b := range []bool{backendModified, loglevelModified, loglevelPresent} {
			if b != s.expectBool[i] {
				t.Fatal("checkReload set an unexpected bool")
			}
		}
	}
}
