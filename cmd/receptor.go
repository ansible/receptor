package main

import (
	"fmt"
	_ "github.com/ghjm/sockceptor/pkg/backends"
	"github.com/ghjm/sockceptor/pkg/cmdline"
	"github.com/ghjm/sockceptor/pkg/debug"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	_ "github.com/ghjm/sockceptor/pkg/services"
	"os"
	"time"
)

var nodeID string

type nodeIDCfg struct {
	NodeID string `description:"Node ID" barevalue:"yes" required:"yes"`
}
func (cfg nodeIDCfg) Prepare() error {
	nodeID = cfg.NodeID
	netceptor.MainInstance = netceptor.New(cfg.NodeID)
	return nil
}

type debugCfg struct {}
func (cfg debugCfg) Prepare() error {
	debug.Enable = true
	return nil
}

type traceCfg struct {}
func (cfg traceCfg) Prepare() error {
	debug.Trace = true
	return nil
}

func main() {
	cmdline.AddConfigType("node-id", "Network node ID of this instance", nodeIDCfg{}, true)
	cmdline.AddConfigType("debug", "Enables debug output", debugCfg{}, false)
	cmdline.AddConfigType("trace", "Enables packet tracing output", traceCfg{}, false)
	cmdline.ParseAndRun(os.Args[1:])

	if nodeID == "" {
		println("Must specify a node ID")
		os.Exit(1)
	}

	/*
	for _, tcpService := range tcpServices {
		debug.Printf("Running TCP service %s\n", tcpService)
		params := strings.Split(tcpService, ":")
		if len(params) != 6 { panic("Invalid parameters for TCP service") }
		go services.TCPProxyService(s, params[0], params[1], params[2], params[3], params[4], params[5])
	}

	for _, udpService := range udpServices {
		debug.Printf("Running UDP service %s\n", udpService)
		params := strings.Split(udpService, ":")
		if len(params) != 6 { panic("Invalid parameters for UDP service") }
		go services.UDPProxyService(s, params[0], params[1], params[2], params[3], params[4], params[5])
	}

	for _, tunService := range tunServices {
		debug.Printf("Running tun service %s\n", tunService)
		params := strings.Split(tunService, ":")
		if len(params) != 4 { panic("Invalid parameters for tun service") }
		go services.TunProxyService(s, params[0], params[1], params[2], params[3])
	}
\	*/

	// Fancy footwork to set an error exitcode if we're immediately exiting at startup
	done := make(chan struct{})
	go func() {
		netceptor.BackendWait()
		close(done)
	}()
	select {
	case <- done:
		if netceptor.BackendCount() > 0 {
			fmt.Printf("All backends have failed. Exiting.\n")
			os.Exit(1)
		} else {
			fmt.Printf("Nothing to do - no backends were specified.\n")
			os.Exit(1)
		}
	case <- time.After(100 * time.Millisecond):
	}
	debug.Printf("Initialization complete\n")
	<- done
}
