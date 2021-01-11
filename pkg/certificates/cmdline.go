package certificates

import "github.com/project-receptor/receptor/pkg/cmdline"

var certSection = &cmdline.ConfigSection{
	Description: "Commands to generate certificates and run a certificate authority",
	Order:       90,
}
