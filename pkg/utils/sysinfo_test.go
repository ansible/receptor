package utils_test

import (
	"context"
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
		commandOutput, err := exec.CommandContext(context.Background(), "nproc").CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to execute nproc command: %v", err)
		}

		commandOutputWithout := strings.TrimSpace(string(commandOutput))
		want, err := strconv.Atoi(commandOutputWithout)
		if err != nil {
			t.Fatalf("Failed to parse CPU count: %v", err)
		}

		if got != want {
			t.Errorf("Expected CPU count: %d, got %d\n", want, got)
		}
	}
}

func TestGetSysMemoryMiB(t *testing.T) {
	got := utils.GetSysMemoryMiB()
	if got <= 0 {
		t.Errorf("Non-positive Memory: %d\n", got)
	}

	if runtime.GOOS == "linux" {
		commandOutput, err := exec.CommandContext(context.Background(), "sed", "-n", "s/^MemTotal:[[:space:]]*\\([[:digit:]]*\\).*/\\1/p", "/proc/meminfo").CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to execute sed command: %v", err)
		}

		commandOutputWithout := strings.TrimSpace(string(commandOutput))
		wantKb, err := strconv.ParseUint(commandOutputWithout, 10, 64)
		if err != nil {
			t.Fatalf("Failed to parse memory size: %v", err)
		}

		want := wantKb / 1024
		if got != want {
			t.Errorf("Expected Memory: %d, got %d\n", want, got)
		}
	}
}
