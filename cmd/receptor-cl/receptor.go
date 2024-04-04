package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"sync"

	_ "github.com/ansible/receptor/internal/version"
	_ "github.com/ansible/receptor/pkg/backends"
	_ "github.com/ansible/receptor/pkg/certificates"
	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/netceptor"
	_ "github.com/ansible/receptor/pkg/services"
	"github.com/ansible/receptor/pkg/types"
	"github.com/ghjm/cmdline"
)

type nullBackendCfg struct{}

// make the nullBackendCfg object be usable as a do-nothing Backend.
func (cfg nullBackendCfg) Start(_ context.Context, _ *sync.WaitGroup) (chan netceptor.BackendSession, error) {
	return make(chan netceptor.BackendSession), nil
}

// Run runs the action, in this case adding a null backend to keep the wait group alive.
func (cfg nullBackendCfg) Run() error {
	err := netceptor.MainInstance.AddBackend(&nullBackendCfg{})
	if err != nil {
		return err
	}

	return nil
}

func (cfg *nullBackendCfg) GetAddr() string {
	return ""
}

func (cfg *nullBackendCfg) GetTLS() *tls.Config {
	return nil
}

func (cfg nullBackendCfg) Reload() error {
	return cfg.Run()
}

func main() {
	cl := cmdline.NewCmdline()
	cl.AddConfigType("node", "Specifies the node configuration of this instance", types.NodeCfg{}, cmdline.Required, cmdline.Singleton)
	cl.AddConfigType("local-only", "Runs a self-contained node with no backend", nullBackendCfg{}, cmdline.Singleton)

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

	osArgs := os.Args[1:]

	err := cl.ParseAndRun(osArgs, []string{"Init", "Prepare", "Run"}, cmdline.ShowHelpIfNoArgs)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
	if cl.WhatRan() != "" {
		// We ran an exclusive command, so we aren't starting any back-ends
		os.Exit(0)
	}

	configPath := ""
	for i, arg := range osArgs {
		if arg == "--config" || arg == "-c" {
			if len(osArgs) > i+1 {
				configPath = osArgs[i+1]
			}

			break
		}
	}

	// only allow reloading if a configuration file was provided. If ReloadCL is
	// not set, then the control service reload command will fail
	if configPath != "" {
		// create closure with the passed in args to be ran during a reload
		reloadParseAndRun := func(toRun []string) error {
			return cl.ParseAndRun(osArgs, toRun)
		}
		err = controlsvc.InitReload(configPath, reloadParseAndRun)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
	}

	if netceptor.MainInstance.BackendCount() == 0 {
		netceptor.MainInstance.Logger.Warning("Nothing to do - no backends are running.\n")
		fmt.Printf("Run %s --help for command line instructions.\n", os.Args[0])
		os.Exit(1)
	}

	netceptor.MainInstance.Logger.Info("Initialization complete\n")

	<-netceptor.MainInstance.NetceptorDone()
}
