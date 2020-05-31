package debug

import (
	"fmt"
)

// Enable controls whether debug messages are printed or not
var Enable bool

// Trace controls whether trace messages are printed or not
var Trace bool

// Printf prints a formatted debug message, if debug enabled
func Printf(format string, a ...interface{}) {
	if Enable {
		fmt.Printf(format, a...)
	}
}

// Tracef prints a formatted trace message, if tracing enabled
func Tracef(format string, a ...interface{}) {
	if Trace {
		fmt.Printf(format, a...)
	}
}
