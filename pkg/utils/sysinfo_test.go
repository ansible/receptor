package utils_test

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/ansible/receptor/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetSysCPUCount(t *testing.T) {
	got := utils.GetSysCPUCount()
	assert.Positive(t, got)

	if runtime.GOOS == "linux" {
		commandOutput, _ := exec.Command("nproc").CombinedOutput()

		commandOutputWithout := strings.TrimSpace(string(commandOutput))
		want, _ := strconv.Atoi(commandOutputWithout)

		assert.Equal(t, got, want)
	}
}

func TestGetSysMemoryMiB(t *testing.T) {
	got := utils.GetSysMemoryMiB()
	assert.Positive(t, got)

	if runtime.GOOS == "linux" {
		commandOutput, _ := exec.Command("sed", "-n", "s/^MemTotal:[[:space:]]*\\([[:digit:]]*\\).*/\\1/p", "/proc/meminfo").CombinedOutput()

		commandOutputWithout := strings.TrimSpace(string(commandOutput))
		wantKb, _ := strconv.ParseUint(commandOutputWithout, 10, 64)

		want := wantKb / 1024
		assert.Equal(t, got, want)
	}
}
