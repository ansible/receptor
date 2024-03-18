package types

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/workceptor"

	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type NodeCfg struct {
	ID                       string                       `description:"Node ID. Defaults to local hostname." barevalue:"yes"`
	DataDir                  string                       `description:"Directory in which to store node data"`
	FirewallRules            []netceptor.FirewallRuleData `description:"Firewall Rules (see documentation for syntax)"`
	MaxIdleConnectionTimeout string                       `description:"Max duration with no traffic before a backend connection is timed out and refreshed."`
}

func (cfg NodeCfg) Init() error {
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

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)

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

	// update netceptor.MainInstance with the MaxIdleConnectionTimeout from the nodeCfg struct
	// this is a fall-forward mechanism. If the user didn't provide a value for MaxIdleConnectionTimeout in their configuration file,
	// we will apply the default timeout of 30s to netceptor.maxConnectionIdleTime
	if cfg.MaxIdleConnectionTimeout != "" {
		err = netceptor.MainInstance.SetMaxConnectionIdleTime(cfg.MaxIdleConnectionTimeout)
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

func (cfg NodeCfg) Run() error {
	workceptor.MainInstance.ListKnownUnitIDs() // Triggers a scan of unit dirs and restarts any that need it

	return nil
}
