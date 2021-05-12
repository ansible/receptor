package backends

import "github.com/ghjm/cmdline"

var backendSection = &cmdline.ConfigSection{
	Description: "Commands to configure back-ends, which connect Receptor nodes together:",
	Order:       10,
}
