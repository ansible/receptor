package main

import (
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
	netceptor.MainInstance = netceptor.New(cfg.ID, allowedPeers)
	controlsvc.MainInstance = controlsvc.New(true, netceptor.MainInstance)
	var err error
	workceptor.MainInstance, err = workceptor.New(controlsvc.MainInstance, netceptor.MainInstance, cfg.DataDir)
	if err != nil {
		return err
	}
	return nil
}

type loglevelCfg struct {
	Level string `description:"Log level to enable Error, Warning, Info, Debug" barevalue:"yes" required:"yes"`
}

func (cfg loglevelCfg) Prepare() error {
	logger.SetLogLevel(logger.GetLogLevelByName(cfg.Level))
	return nil
}

type traceCfg struct{}

func (cfg traceCfg) Prepare() error {
	logger.SetShowTrace(true)
	return nil
}

type nullBackendCfg struct{}

func (cfg nullBackendCfg) Run() error {
	// This is a null backend that doesn't do anything
	netceptor.AddBackend()
	return nil
}

func main() {
	cmdline.AddConfigType("node", "Node configuration of this instance", nodeCfg{}, true, false, false, nil)
	cmdline.AddConfigType("log-level", "Set specific log level output", loglevelCfg{}, false, false, false, nil)
	cmdline.AddConfigType("trace", "Enables packet tracing output", traceCfg{}, false, false, false, nil)
	cmdline.AddConfigType("local-only", "Run a self-contained node with no backends", nullBackendCfg{}, false, false, false, nil)
	cmdline.ParseAndRun(os.Args[1:])

	// Fancy footwork to set an error exitcode if we're immediately exiting at startup
	done := make(chan struct{})
	go func() {
		netceptor.BackendWait()
		close(done)
	}()
	select {
	case <-done:
		if netceptor.BackendCount() > 0 {
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
