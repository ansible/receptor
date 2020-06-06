package backends

import "github.com/ghjm/sockceptor/pkg/cmdline"

var backendSection = &cmdline.Section{
	Description: "Commands to configure back-ends, which connect Receptor nodes together:",
	Order:       10,
}
