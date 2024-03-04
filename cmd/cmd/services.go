package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	controlServiceReceptorService string
	controlServiceFilename        string
	controlServicePermissions     int
	controlServiceTLS             string
	controlServiceTCPListen       string
	controlServiceTCPTLS          string
	commandServiceService         string
	commandServiceCommand         string
	commandServiceTLS             string
	ipRouterNetworkName           string
	ipRouterInterface             string
	ipRouterLocalNet              string
	ipRouterRoutes                string
	tcpServerPort                 int
	tcpServerBindAddr             string
	tcpServerRemoteNode           string
	tcpServerRemoteService        string
	tcpServerTLSServer            string
	tcpServerTLSClient            string
	tcpClientService              string
	tcpClientAddress              string
	tcpClientTLSServer            string
	tcpClientTLSClient            string
	udpServerPort                 int
	udpServerBindAddr             string
	udpServerRemoteNode           string
	udpServerRemoteService        string
	udpClientService              string
	udpClientAddress              string
	unixSocketServerFilename      string
	unixSocketServerPermissions   int
	unixSocketServerRemoteNode    string
	unixSocketServerRemoteService string
	unixSocketServerTLS           string
	unixSocketClientService       string
	unixSocketClientFilename      string
	unixSocketClientTLS           string
)

var controlService = &cobra.Command{
	Use:   "control-service",
	Short: "Run a control service",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var commandService = &cobra.Command{
	Use:   "command-service",
	Short: "Run an interactive command via a Receptor service",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var ipRouter = &cobra.Command{
	Use:   "ip-router",
	Short: "Run an IP router using a tun interface",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var tcpServer = &cobra.Command{
	Use:   "tcp-server",
	Short: "Listen for TCP and forward via Receptor",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var tcpClient = &cobra.Command{
	Use:   "tcp-client",
	Short: "Listen on a Receptor service and forward via TCP",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var udpServer = &cobra.Command{
	Use:   "udp-server",
	Short: "Listen for UDP and forward via Receptor",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var udpClient = &cobra.Command{
	Use:   "udp-client",
	Short: "Listen on a Receptor service and forward via UDP",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var unixSocketServer = &cobra.Command{
	Use:   "unix-socket-server",
	Short: "Listen on a Unix socket and forward via Receptor",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var unixSocketClient = &cobra.Command{
	Use:   "unix-socket-client",
	Short: "Listen via Receptor and forward to a Unix socket",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

func init() {
	controlService.Flags().StringVar(&controlServiceReceptorService, "service", "control", "Receptor service name to listen on (default: control)")
	controlService.Flags().StringVar(&controlServiceFilename, "filename", "", "Filename of local Unix socket to bind to the service")
	controlService.Flags().IntVar(&controlServicePermissions, "permissions", 0600, "Socket file permissions (default: 0600)")
	controlService.Flags().StringVar(&controlServiceTLS, "tls", "", "Name of TLS server config for the Receptor listener")
	controlService.Flags().StringVar(&controlServiceTCPListen, "tcplisten", "", "Local TCP port or host:port to bind to the control service")
	controlService.Flags().StringVar(&controlServiceTCPTLS, "tcptls", "", "Name of TLS server config for the TCP listener")

	commandService.Flags().StringVar(&commandServiceService, "service", "", "Receptor service name to bind to (required)")
	commandService.MarkFlagRequired("service")
	commandService.Flags().StringVar(&commandServiceCommand, "command", "", "Command to execute on a connection (required)")
	commandService.MarkFlagRequired("command")
	commandService.Flags().StringVar(&commandServiceTLS, "tls", "", "Name of TLS server config")

	ipRouter.Flags().StringVar(&ipRouterNetworkName, "networkname", "", "Name of this network and service. (required)")
	ipRouter.MarkFlagRequired("networkname")
	ipRouter.Flags().StringVar(&ipRouterInterface, "interface", "", "Name of the local tun interface")
	ipRouter.Flags().StringVar(&ipRouterLocalNet, "localnet", "", "Local /30 CIDR address (required)")
	ipRouter.MarkFlagRequired("localnet")
	// Consider changing this one to a slice?
	ipRouter.Flags().StringVar(&ipRouterRoutes, "routes", "", "Comma separated list of CIDR subnets to advertise")

	tcpServer.Flags().IntVar(&tcpServerPort, "port", 0, "Local TCP port to bind to (required)")
	tcpServer.MarkFlagRequired("port")
	tcpServer.Flags().StringVar(&tcpServerBindAddr, "bindaddr", "0.0.0.0", "Address to bind TCP listener to (default: 0.0.0.0)")
	tcpServer.Flags().StringVar(&tcpServerRemoteNode, "remotenode", "", "Receptor node to connect to (required)")
	tcpServer.MarkFlagRequired("remotenode")
	tcpServer.Flags().StringVar(&tcpServerRemoteService, "remoteservice", "", "Receptor service name to connect to (required)")
	tcpServer.MarkFlagRequired("remoteservice")
	tcpServer.Flags().StringVar(&tcpServerTLSServer, "tlsserver", "", "Name of TLS server config for the TCP listener")
	tcpServer.Flags().StringVar(&tcpServerTLSClient, "tlsclient", "", "Name of TLS client config for the Receptor connection")

	tcpClient.Flags().StringVar(&tcpClientService, "service", "", "Receptor service name to bind to (required)")
	tcpClient.MarkFlagRequired("service")
	tcpClient.Flags().StringVar(&tcpClientAddress, "address", "", "Address for outbound TCP connection (required)")
	tcpClient.MarkFlagRequired("address")
	tcpClient.Flags().StringVar(&tcpClientTLSServer, "tlsserver", "", "Name of TLS server config for the Receptor service")
	tcpClient.Flags().StringVar(&tcpClientTLSClient, "tlsclient", "", "Name of TLS client config for the TCP connection")

	udpServer.Flags().IntVar(&udpServerPort, "port", 0, "Local UDP port to bind to (required)")
	udpServer.MarkFlagRequired("port")
	udpServer.Flags().StringVar(&udpServerBindAddr, "bindaddr", "0.0.0.0", "Address to bind UDP listener to (default: 0.0.0.0)")
	udpServer.Flags().StringVar(&udpServerRemoteNode, "remotenode", "", "Receptor node to connect to (required)")
	udpServer.MarkFlagRequired("remotenode")
	udpServer.Flags().StringVar(&udpServerRemoteService, "remoteservice", "", "Receptor service name to connect to (required)")
	udpServer.MarkFlagRequired("remoteservice")

	udpClient.Flags().StringVar(&udpClientService, "service", "", "Receptor service name to bind to (required)")
	udpClient.MarkFlagRequired("service")
	udpClient.Flags().StringVar(&udpClientAddress, "address", "", "Address for outbound UDP connection (required)")
	udpClient.MarkFlagRequired("address")

	unixSocketServer.Flags().StringVar(&unixSocketServerFilename, "filename", "", "Socket filename, which will be overwritten (required)")
	unixSocketServer.MarkFlagRequired("filename")
	unixSocketServer.Flags().IntVar(&unixSocketServerPermissions, "permissions", 0600, "Socket file permissions (default: 0600)")
	unixSocketServer.Flags().StringVar(&unixSocketServerRemoteNode, "remotenode", "", "Receptor node to connect to (required)")
	unixSocketServer.MarkFlagRequired("remotenode")
	unixSocketServer.Flags().StringVar(&unixSocketServerRemoteService, "remoteservice", "", "Receptor service name to connect to (required)")
	unixSocketServer.MarkFlagRequired("remoteservice")
	unixSocketServer.Flags().StringVar(&unixSocketServerTLS, "tls", "", "Name of TLS client config for the Receptor connection")

	unixSocketClient.Flags().StringVar(&unixSocketClientService, "service", "", "Receptor service name to bind to (required)")
	unixSocketClient.MarkFlagRequired("service")
	unixSocketClient.Flags().StringVar(&unixSocketClientFilename, "filename", "", "Socket filename, which must already exist (required)")
	unixSocketClient.MarkFlagRequired("filename")
	unixSocketClient.Flags().StringVar(&unixSocketClientTLS, "tls", "", "Name of TLS server config for the Receptor connection")

	rootCmd.AddCommand(controlService, commandService, ipRouter, tcpServer, tcpClient, udpServer, udpClient, unixSocketServer, unixSocketClient)

}
