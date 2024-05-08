package cmd

import (
	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/types"
)

type Config struct {
	Node           types.NodeCfg
	LogLevel       logger.LoglevelCfg           `mapstructure:"log-level"`
	ControlService controlsvc.CmdlineConfigUnix `mapstructure:"control-service"`
	TCPPeer        backends.TCPDialerCfg        `mapstructure:"tcp-peer"`
}
