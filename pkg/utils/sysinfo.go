package utils

import (
	"runtime"

	"github.com/pbnjay/memory"
)

// GetSysCPUCount returns number of logical CPU cores on the system
func GetSysCPUCount() int {
	return runtime.NumCPU()
}

// GetSysMemoryMiB returns the capacity (in mebibytes) of the physical memory installed on the system
func GetSysMemoryMiB() uint64 {
	return memory.TotalMemory() / 1048576 // bytes to MiB
}
