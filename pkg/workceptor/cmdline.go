//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import "github.com/ghjm/cmdline"

var workersSection = &cmdline.ConfigSection{
	Description: "Commands to configure workers that process units of work:",
	Order:       30,
}
