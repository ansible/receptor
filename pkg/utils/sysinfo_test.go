package utils_test

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/ansible/receptor/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetSysCPUCount(t *testing.T) {
	got := utils.GetSysCPUCount()
	assert.Positive(t, got)

	commandOutput, err1 := exec.Command("nproc").CombinedOutput()

	assert.Nilf(t, err1, "Unable to get nproc output, output = %s, err = %s", string(commandOutput), err1)

	commandOutputWithout := strings.TrimSpace(string(commandOutput))
	want, err2 := strconv.Atoi(commandOutputWithout)
	assert.Nilf(t, err2, "Unable to convert nproc output %s, err = %s", string(commandOutput), err2)

	assert.Equal(t, got, want)
}

func TestGetSysMemoryMiB(t *testing.T) {
	got := utils.GetSysMemoryMiB()
	assert.Positive(t, got)

	commandOutput, err1 := exec.Command("sed", "-n", "s/^MemTotal:[[:space:]]*\\([[:digit:]]*\\).*/\\1/p", "/proc/meminfo").CombinedOutput()
	assert.Nilf(t, err1, "Unable to get /proc/meminfo output, err = %s", err1)

	commandOutputWithout := strings.TrimSpace(string(commandOutput))
	wantKb, err2 := strconv.ParseUint(commandOutputWithout, 10, 64)
	assert.Nilf(t, err2, "Unable to convert /proc/meminfo output %s, err = %s", string(commandOutput), err2)

	want := wantKb / 1024
	assert.Equal(t, got, want)
}
