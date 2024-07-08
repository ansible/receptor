package cmd

func SetTCPListenerDefaults(config *ReceptorConfig) {
	for _, listener := range config.TCPListeners {
		if listener.Cost == 0 {
			listener.Cost = 1.0
		}
		if listener.BindAddr == "" {
			listener.BindAddr = "0.0.0.0"
		}
	}
}

func SetUDPListenerDefaults(config *ReceptorConfig) {
	for _, listener := range config.UDPListeners {
		if listener.Cost == 0 {
			listener.Cost = 1.0
		}
		if listener.BindAddr == "" {
			listener.BindAddr = "0.0.0.0"
		}
	}
}

func SetWSListenerDefaults(config *ReceptorConfig) {
	for _, listener := range config.WSListeners {
		if listener.Cost == 0 {
			listener.Cost = 1.0
		}
		if listener.BindAddr == "" {
			listener.BindAddr = "0.0.0.0"
		}
		if listener.Path == "" {
			listener.BindAddr = "/"
		}
	}
}

func SetUDPPeerDefaults(config *ReceptorConfig) {
	for _, peer := range config.TCPPeers {
		if peer.Cost == 0 {
			peer.Cost = 1.0
		}

		if !peer.Redial {
			peer.Redial = true
		}
	}
}

func SetTCPPeerDefaults(config *ReceptorConfig) {
	for _, peer := range config.TCPPeers {
		if peer.Cost == 0 {
			peer.Cost = 1.0
		}

		if !peer.Redial {
			peer.Redial = true
		}
	}
}

func SetWSPeerDefaults(config *ReceptorConfig) {
	for _, peer := range config.WSPeers {
		if peer.Cost == 0 {
			peer.Cost = 1.0
		}

		if !peer.Redial {
			peer.Redial = true
		}
	}
}

func SetCmdlineUnixDefaults(config *ReceptorConfig) {
	for _, service := range config.ControlServices {
		if service.Permissions == 0 {
			service.Permissions = 0o600
		}

		if service.Service == "" {
			service.Service = "control"
		}
	}
}

func SetLogLevelDefaults(config *ReceptorConfig) {
	if config.LogLevel.Level == "" {
		config.LogLevel.Level = "error"
	}
}

func SetNodeDefaults(config *ReceptorConfig) {
	if config.Node.DataDir == "" {
		config.Node.DataDir = "/tmp/receptor"
	}
}

func SetKubeWorkerDefaults(config *ReceptorConfig) {
	for _, worker := range config.WorkKubernetes {
		if worker.AuthMethod == "" {
			worker.AuthMethod = "incluster"
		}

		if worker.StreamMethod == "" {
			worker.StreamMethod = "logger"
		}
	}
}

func SetConfigDefaults(config *ReceptorConfig) {
	SetTCPListenerDefaults(config)
	SetUDPListenerDefaults(config)
	SetWSListenerDefaults(config)
	SetTCPPeerDefaults(config)
	SetUDPPeerDefaults(config)
	SetWSPeerDefaults(config)
	SetCmdlineUnixDefaults(config)
	SetLogLevelDefaults(config)
	SetNodeDefaults(config)
	SetKubeWorkerDefaults(config)
}
