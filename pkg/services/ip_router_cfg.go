package services

// ipRouterCfg is the cmdline configuration object for an IP router.
type IPRouterCfg struct {
	NetworkName string `required:"true" description:"Name of this network and service."`
	Interface   string `description:"Name of the local tun interface"`
	LocalNet    string `required:"true" description:"Local /30 CIDR address"`
	Routes      string `description:"Comma separated list of CIDR subnets to advertise"`
}
