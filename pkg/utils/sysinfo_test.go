package utils_test

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/ansible/receptor/pkg/utils"
)

func TestGetSysCPUCount(t *testing.T) {
	got := utils.GetSysCPUCount()
	if got <= 0 {
		t.Errorf("Non-positive CPU count: %d\n", got)
	}

	if runtime.GOOS == "linux" {
		commandOutput, _ := exec.Command("nproc").CombinedOutput()

		commandOutputWithout := strings.TrimSpace(string(commandOutput))
		want, _ := strconv.Atoi(commandOutputWithout)

		if got != want {
			t.Errorf("Expected CPU count: %d, got %d\n", want, got)
		}
	}
}

func TestGetSysMemoryMiB(t *testing.T) {
	got := utils.GetSysMemoryMiB()
	if got <= 0 {
		t.Errorf( "Non-positive Memory: %d\n", got)
	}

	if runtime.GOOS == "linux" {
		commandOutput, _ := exec.Command("sed", "-n", "s/^MemTotal:[[:space:]]*\\([[:digit:]]*\\).*/\\1/p", "/proc/meminfo").CombinedOutput()

		commandOutputWithout := strings.TrimSpace(string(commandOutput))
		wantKb, _ := strconv.ParseUint(commandOutputWithout, 10, 64)

		want := wantKb / 1024
		if got != want {
			t.Errorf( "Expected Memory: %d, got %d\n", want, got )
		}
	}
}
