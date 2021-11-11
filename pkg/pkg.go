package pkg

import (
	"context"
	"errors"
	"fmt"

	"github.com/ansible/receptor/pkg/backends"
	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/services"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/ansible/receptor/pkg/workceptor"
)

// ErrNoBackends indicates that no backends were specified for a receptor instance.
var ErrNoBackends = errors.New("no backends were specified in serve config")

// Receptor defines the configuration for a receptor instance.
type Receptor struct {
	// Overrides the default loglevel on the root logger.
	LogLevel *string `mapstructure:"log-level"`
	// Enable receptor packet tracing.
	EnableTracing bool `mapstructure:"enable-tracing"`
	// Node ID. Defaults to local hostname.
	ID *string `mapstructure:"id"`
	// List of peer node-IDs to allow.
	AllowedPeers []string `mapstructure:"allowed-peers"`
	// Directory in which to store node data.
	DataDir     string                  `mapstructure:"data-dir"`
	Backends    *backends.Backends      `mapstructure:"backends"`
	Services    *services.Services      `mapstructure:"services"`
	Workers     *workceptor.Workers     `mapstructure:"workers"`
	Controllers *controlsvc.Controllers `mapstructure:"controllers"`
}

// Serve launches an receptor instance and blocks until canceled or failed.
func (r Receptor) Serve(ctx context.Context) error {
	logger.SetShowTrace(r.EnableTracing)

	if r.LogLevel != nil {
		val, err := logger.GetLogLevelByName(*r.LogLevel)
		if err != nil {
			return fmt.Errorf("log level in serve config is invalid: %w", err)
		}
		logger.SetLogLevel(val)
	}

	var id string
	var err error
	if r.ID == nil {
		id = utils.GenerateHostID()
	} else {
		id = *r.ID
	}

	nc := netceptor.New(ctx, id)
	wc, err := workceptor.New(ctx, nc, r.DataDir)
	if err != nil {
		return fmt.Errorf("could not setup workceptor from serve config: %w", err)
	}

	cv := controlsvc.New(true, nc)

	if r.Backends != nil {
		if err := r.Backends.Setup(nc); err != nil {
			return fmt.Errorf("could not setup listeners from serve config: %w", err)
		}
	}

	if r.Services != nil {
		if err := r.Services.Setup(nc); err != nil {
			return fmt.Errorf("could not setup services from serve config: %w", err)
		}
	}

	if r.Workers != nil {
		if err := r.Workers.Setup(wc); err != nil {
			return fmt.Errorf("could not setup workers from serve config: %w", err)
		}
	}

	if r.Controllers != nil {
		if err := r.Controllers.Setup(ctx, cv); err != nil {
			return fmt.Errorf("could not setup controllers from serve config: %w", err)
		}
	}

	// crutch to ensure refresh.
	wc.ListKnownUnitIDs()

	if nc.BackendCount() < 1 {
		return ErrNoBackends
	}

	nc.BackendWait()

	return nil
}
