package controlsvc

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.NoError(t, err)
	assert.Len(t, cfgNotReloadable, 5)

	for _, s := range scenarios {
		t.Logf("%s", s.filename)
		err = parseConfigForReload(s.filename, true)
		if s.modifyError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
		err = cfgAbsent()
		if s.absentError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}
