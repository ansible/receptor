package services

import "github.com/ghjm/sockceptor/pkg/cmdline"

var servicesSection = &cmdline.Section{
	Description: "Commands to configure services that run on top of the Receptor mesh:",
	Order:       20,
}
