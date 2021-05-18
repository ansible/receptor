package certificates

import "github.com/ghjm/cmdline"

var certSection = &cmdline.ConfigSection{
	Description: "Commands to generate certificates and run a certificate authority",
	Order:       90,
}
