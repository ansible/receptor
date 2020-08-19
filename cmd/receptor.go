package main

import (
	"context"
	"fmt"
	_ "github.com/project-receptor/receptor/pkg/backends"
	"github.com/project-receptor/receptor/pkg/cmdline"
	"github.com/project-receptor/receptor/pkg/controlsvc"
	_ "github.com/project-receptor/receptor/pkg/controlsvc"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	_ "github.com/project-receptor/receptor/pkg/services"
	"github.com/project-receptor/receptor/pkg/workceptor"
	_ "github.com/project-receptor/receptor/pkg/workceptor"
	"os"
	"strings"
	"time"
)

type nodeCfg struct {
	ID           string `description:"Node ID" barevalue:"yes" required:"yes"`
	AllowedPeers string `description:"Comma separated list of peer node-IDs to allow" required:"no"`
	DataDir      string `description:"Directory in which to store node data"`
}

func (cfg nodeCfg) Prepare() error {
	var allowedPeers []string
	if cfg.AllowedPeers != "" {
		allowedPeers = strings.Split(cfg.AllowedPeers, ",")
	}
	netceptor.MainInstance = netceptor.New(context.Background(), cfg.ID, allowedPeers)
	controlsvc.MainInstance = controlsvc.New(true, netceptor.MainInstance)
	var err error
	workceptor.MainInstance, err = workceptor.New(controlsvc.MainInstance, netceptor.MainInstance, cfg.DataDir)
	if err != nil {
		return err
	}
	return nil
}

type nullBackendCfg struct{}

// make the nullBackendCfg object be usable as a do-nothing Backend
func (cfg nullBackendCfg) Start(ctx context.Context) (chan netceptor.BackendSession, error) {
	return make(chan netceptor.BackendSession), nil
}

// Run runs the action, in this case adding a null backend to keep the wait group alive
func (cfg nullBackendCfg) Run() error {
	err := netceptor.MainInstance.AddBackend(&nullBackendCfg{}, 1.0, nil)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	cmdline.AddConfigType("node", "Node configuration of this instance", nodeCfg{}, true, false, false, nil)
	cmdline.AddConfigType("local-only", "Run a self-contained node with no backends", nullBackendCfg{}, false, false, false, nil)
	cmdline.ParseAndRun(os.Args[1:])

	// Fancy footwork to set an error exitcode if we're immediately exiting at startup
	done := make(chan struct{})
	go func() {
		netceptor.MainInstance.BackendWait()
		close(done)
	}()
	select {
	case <-done:
		if netceptor.MainInstance.BackendCount() > 0 {
			logger.Error("All backends have failed. Exiting.\n")
			os.Exit(1)
		} else {
			logger.Warning("Nothing to do - no backends were specified.\n")
			fmt.Printf("Run %s --help for command line instructions.\n", os.Args[0])
			os.Exit(1)
		}
	case <-time.After(100 * time.Millisecond):
	}
	logger.Info("Initialization complete\n")
	<-done
}
