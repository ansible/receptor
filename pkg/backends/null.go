package backends

import (
	"context"
	"crypto/tls"
	"sync"

	"github.com/ansible/receptor/pkg/netceptor"
)

type NullBackendCfg struct{}

// make the nullBackendCfg object be usable as a do-nothing Backend.
func (cfg NullBackendCfg) Start(_ context.Context, _ *sync.WaitGroup) (chan netceptor.BackendSession, error) {
	return make(chan netceptor.BackendSession), nil
}

// Run runs the action, in this case adding a null backend to keep the wait group alive.
func (cfg NullBackendCfg) Run() error {
	err := netceptor.MainInstance.AddBackend(&NullBackendCfg{})
	if err != nil {
		return err
	}

	return nil
}

func (cfg *NullBackendCfg) GetAddr() string {
	return ""
}

func (cfg *NullBackendCfg) GetTLS() *tls.Config {
	return nil
}

func (cfg NullBackendCfg) Reload() error {
	return cfg.Run()
}
