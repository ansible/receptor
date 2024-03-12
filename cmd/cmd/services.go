package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// servicesCmd represents the services command
var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("services called")
	},
}

func init() {
	rootCmd.AddCommand(servicesCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// servicesCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// servicesCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Commands to configure services that run on top of the Receptor mesh:

//    --command-service: Run an interactive command via a Receptor service
//       service=<string>: Receptor service name to bind to (required)
//       command=<string>: Command to execute on a connection (required)
//       tls=<string>: Name of TLS server config

//    --ip-router: Run an IP router using a tun interface
//       networkname=<string>: Name of this network and service. (required)
//       interface=<string>: Name of the local tun interface
//       localnet=<string>: Local /30 CIDR address (required)
//       routes=<string>: Comma separated list of CIDR subnets to advertise

//    --tcp-server: Listen for TCP and forward via Receptor
//       port=<int>: Local TCP port to bind to (required)
//       bindaddr=<string>: Address to bind TCP listener to (default: 0.0.0.0)
//       remotenode=<string>: Receptor node to connect to (required)
//       remoteservice=<string>: Receptor service name to connect to (required)
//       tlsserver=<string>: Name of TLS server config for the TCP listener
//       tlsclient=<string>: Name of TLS client config for the Receptor connection

//    --tcp-client: Listen on a Receptor service and forward via TCP
//       service=<string>: Receptor service name to bind to (required)
//       address=<string>: Address for outbound TCP connection (required)
//       tlsserver=<string>: Name of TLS server config for the Receptor service
//       tlsclient=<string>: Name of TLS client config for the TCP connection

//    --udp-server: Listen for UDP and forward via Receptor
//       port=<int>: Local UDP port to bind to (required)
//       bindaddr=<string>: Address to bind UDP listener to (default: 0.0.0.0)
//       remotenode=<string>: Receptor node to connect to (required)
//       remoteservice=<string>: Receptor service name to connect to (required)

//    --udp-client: Listen on a Receptor service and forward via UDP
//       service=<string>: Receptor service name to bind to (required)
//       address=<string>: Address for outbound UDP connection (required)

//    --unix-socket-server: Listen on a Unix socket and forward via Receptor
//       filename=<string>: Socket filename, which will be overwritten (required)
//       permissions=<int>: Socket file permissions (default: 0600)
//       remotenode=<string>: Receptor node to connect to (required)
//       remoteservice=<string>: Receptor service name to connect to (required)
//       tls=<string>: Name of TLS client config for the Receptor connection

//    --unix-socket-client: Listen via Receptor and forward to a Unix socket
//       service=<string>: Receptor service name to bind to (required)
//       filename=<string>: Socket filename, which must already exist (required)
//       tls=<string>: Name of TLS server config for the Receptor connection
