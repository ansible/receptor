package utils

import (
	"github.com/pbnjay/memory"
	"runtime"
)

// GetSysCPUCores returns number of CPU cores on the system
func GetSysCPUCores() int {
	return runtime.NumCPU()
}

// GetSysMemoryMB returns the capacity (in MB) of the physical memory installed on the system
func GetSysMemoryMB() uint {
	return uint(memory.TotalMemory() / 1e6)
}
