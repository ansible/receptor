package debug

import (
	"fmt"
)

var Enable bool
var Trace bool

func Printf(format string, a ...interface{}) {
	if Enable {
		fmt.Printf(format, a...)
	}
}

func Tracef(format string, a ...interface{}) {
	if Trace {
		fmt.Printf(format, a...)
	}
}