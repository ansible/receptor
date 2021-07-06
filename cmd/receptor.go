package main

import (
	"context"
	"fmt"
	"github.com/ghjm/cmdline"
	_ "github.com/project-receptor/receptor/pkg/backends"
	_ "github.com/project-receptor/receptor/pkg/certificates"
	"github.com/project-receptor/receptor/pkg/controlsvc"
	"github.com/project-receptor/receptor/pkg/logger"
	"github.com/project-receptor/receptor/pkg/netceptor"
	_ "github.com/project-receptor/receptor/pkg/services"
	_ "github.com/project-receptor/receptor/pkg/version"
	"github.com/project-receptor/receptor/pkg/workceptor"
	"os"
	"strings"
	"time"
)

type nodeCfg struct {
	ID            string   `description:"Node ID. Defaults to local hostname." barevalue:"yes"`
	DataDir       string   `description:"Directory in which to store node data"`
	FirewallRules []string `description:"Firewall Rules (see documentation for syntax)"`
}

func (cfg nodeCfg) Init() error {
	var err error
	if cfg.ID == "" {
		host, err := os.Hostname()
		if err != nil {
			return err
		}
		lchost := strings.ToLower(host)
		if lchost == "localhost" || strings.HasPrefix(lchost, "localhost.") {
			return fmt.Errorf("no node ID specified and local host name is localhost")
		}
		cfg.ID = host
	}
	if strings.ToLower(cfg.ID) == "localhost" {
		return fmt.Errorf("node ID \"localhost\" is reserved")
	}
	netceptor.MainInstance = netceptor.New(context.Background(), cfg.ID)
	if len(cfg.FirewallRules) > 0 {
		rules, err := netceptor.ParseFirewallRules(cfg.FirewallRules)
		if err != nil {
			return err
		}
		err = netceptor.MainInstance.AddFirewallRules(rules, true)
		if err != nil {
			return err
		}
	}
	workceptor.MainInstance, err = workceptor.New(context.Background(), netceptor.MainInstance, cfg.DataDir)
	if err != nil {
		return err
	}
	controlsvc.MainInstance = controlsvc.New(true, netceptor.MainInstance)
	err = workceptor.MainInstance.RegisterWithControlService(controlsvc.MainInstance)
	if err != nil {
		return err
	}
	return nil
}

func (cfg nodeCfg) Run() error {
	workceptor.MainInstance.ListKnownUnitIDs() // Triggers a scan of unit dirs and restarts any that need it
	return nil
}

type nullBackendCfg struct{}

// Start makes the nullBackendCfg object usable as a do-nothing Backend
func (cfg nullBackendCfg) Start(_ context.Context) (chan netceptor.BackendSession, error) {
	return make(chan netceptor.BackendSession), nil
}

// Run runs the action, in this case adding a null backend to keep the wait group alive
func (cfg nullBackendCfg) Run() error {
	err := netceptor.MainInstance.AddBackend(&nullBackendCfg{})
	if err != nil {
		return err
	}
	return nil
}

func main() {
	cl := cmdline.NewCmdline()
	cl.AddConfigType("node", "Node configuration of this instance", nodeCfg{}, cmdline.Required, cmdline.Singleton)
	cl.AddConfigType("local-only", "Run a self-contained node with no backends", nullBackendCfg{}, cmdline.Singleton)

	// Add registered config types from imported modules
	for _, appName := range []string{
		"receptor-version",
		"receptor-logging",
		"receptor-tls",
		"receptor-certificates",
		"receptor-control-service",
		"receptor-command-service",
		"receptor-ip-router",
		"receptor-proxies",
		"receptor-backends",
		"receptor-workers",
	} {
		cl.AddRegisteredConfigTypes(appName)
	}

	err := cl.ParseAndRun(os.Args[1:], []string{"Init", "Prepare", "Run"}, cmdline.ShowHelpIfNoArgs)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
	if cl.WhatRan() != "" {
		// We ran an exclusive command, so we aren't starting any back-ends
		os.Exit(0)
	}

	// Fancy footwork to set an error exit code if we're immediately exiting at startup
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
