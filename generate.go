package main

//go:generate mockgen -source=pkg/backends/websockets.go -destination=pkg/backends/mock_backends/websockets.go
//go:generate mockgen -source=pkg/certificates/oser.go -destination=pkg/certificates/mock_certificates/oser.go
//go:generate mockgen -source=pkg/certificates/rsaer.go -destination=pkg/certificates/mock_certificates/rsaer.go
//go:generate mockgen -source=pkg/controlsvc/controlsvc.go -destination=pkg/controlsvc/mock_controlsvc/controlsvc.go
//go:generate mockgen -source=pkg/controlsvc/interfaces.go -destination=pkg/controlsvc/mock_controlsvc/interfaces.go
//go:generate mockgen -source=pkg/framer/framer.go -destination=pkg/framer/mock_framer/framer.go
//go:generate mockgen -source=pkg/netceptor/conn.go -destination=pkg/netceptor/mock_netceptor/conn.go
//go:generate mockgen -source=pkg/netceptor/external_backend.go -destination=pkg/netceptor/mock_netceptor/external_backend.go
//go:generate mockgen -source=pkg/netceptor/netceptor.go -destination=pkg/netceptor/mock_netceptor/netceptor.go
//go:generate mockgen -source=pkg/netceptor/packetconn.go -destination=pkg/netceptor/mock_netceptor/packetconn.go
//go:generate mockgen -source=pkg/netceptor/ping.go -destination=pkg/netceptor/mock_netceptor/ping.go
//go:generate mockgen -source=pkg/services/interfaces/net_interfaces.go -destination=pkg/services/interfaces/mock_interfaces/net_interfaces.go
//go:generate mockgen -source=pkg/services/command.go -destination=pkg/services/mock_services/command.go
//go:generate mockgen -source=pkg/services/tcp_proxy.go -destination=pkg/services/mock_services/tcp_proxy.go
//go:generate mockgen -source=pkg/services/udp_proxy.go -destination=pkg/services/mock_services/udp_proxy.go
//go:generate mockgen -source=pkg/utils/net.go -destination=pkg/utils/mock_utils/net.go
//go:generate mockgen -source=pkg/workceptor/command.go -destination=pkg/workceptor/mock_workceptor/command.go
//go:generate mockgen -source=pkg/workceptor/interfaces.go -destination=pkg/workceptor/mock_workceptor/interfaces.go
//go:generate mockgen -source=pkg/workceptor/kubernetes.go -destination=pkg/workceptor/mock_workceptor/kubernetes.go
//go:generate mockgen -source=pkg/workceptor/stdio_utils.go -destination=pkg/workceptor/mock_workceptor/stdio_utils.go
//go:generate mockgen -source=pkg/workceptor/workceptor.go -destination=pkg/workceptor/mock_workceptor/workceptor.go
//go:generate mockgen -source=pkg/workceptor/workunitbase.go -destination=pkg/workceptor/mock_workceptor/workunitbase.go
