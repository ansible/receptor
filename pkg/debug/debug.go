package debug

import (
	"fmt"
)

var Enable bool

func Printf(format string, a ...interface{}) {
	if Enable {
		fmt.Printf(format, a...)
	}
}
