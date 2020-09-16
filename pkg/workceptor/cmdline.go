// +build !no_workceptor

package workceptor

import "github.com/project-receptor/receptor/pkg/cmdline"

var workersSection = &cmdline.ConfigSection{
	Description: "Commands to configure workers that process units of work:",
	Order:       30,
}
