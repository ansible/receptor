package controlsvc

import (
	"testing"
)

func TestReload(t *testing.T) {
	type yamltest struct {
		filename    string
		modifyError bool
		absentError bool
	}

	scenarios := []yamltest{
		{filename: "reload_test_yml/init.yml", modifyError: false, absentError: false},
		{filename: "reload_test_yml/add_cfg.yml", modifyError: true, absentError: false},
		{filename: "reload_test_yml/drop_cfg.yml", modifyError: false, absentError: true},
		{filename: "reload_test_yml/modify_cfg.yml", modifyError: true, absentError: true},
		{filename: "reload_test_yml/syntax_error.yml", modifyError: true, absentError: true},
		{filename: "reload_test_yml/successful_reload.yml", modifyError: false, absentError: false},
	}
	err := parseConfigForReload("reload_test_yml/init.yml", false)
	if err != nil {
		t.Errorf("parseConfigForReload %s: Unexpected err: %v", "init.yml", err)
	}

	if len(cfgNotReloadable) != 5 {
		t.Errorf("cfNotReloadable length expected %d, got %d", 5, len(cfgNotReloadable))
	}

	for _, s := range scenarios {
		t.Logf("%s", s.filename)
		err = parseConfigForReload(s.filename, true)
		if s.modifyError {
			if err == nil {
				t.Errorf("parseConfigForReload %s %s: Expected err, got %v", s.filename, "modifyError", err)
			}
		} else {
			if err != nil {
				t.Errorf("parseConfigForReload %s %s: Unexpected err: %v", s.filename, "modifyError", err)
			}
		}
		err = cfgAbsent()
		if s.absentError {
			if err == nil {
				t.Errorf("parseConfigForReload %s %s: Expected err, got %v", s.filename, "absentError", err)
			}
		} else {
			if err != nil {
				t.Errorf("parseConfigForReload %s %s: Unexpected err: %v", s.filename, "absentError", err)
			}
		}
	}
}
