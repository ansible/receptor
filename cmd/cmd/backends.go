package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// backendsCmd represents the backends command
var backendsCmd = &cobra.Command{
	Use:   "backends",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("backends called")
	},
}

func init() {
	rootCmd.AddCommand(backendsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// backendsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// backendsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Commands to configure back-ends, which connect Receptor nodes together:

//    --tcp-listener: Run a backend listener on a TCP port
//       bindaddr=<string>: Local address to bind to (default: 0.0.0.0)
//       port=<int>: Local TCP port to listen on (required)
//       tls=<string>: Name of TLS server config
//       cost=<float64>: Connection cost (weight) (default: 1.0)
//       nodecost=<JSON dict of string to float64>: Per-node costs
//       allowedpeers=<[]string (may be repeated)>: Peer node IDs to allow via this connection

//    --tcp-peer: Make an outbound backend connection to a TCP peer
//       address=<string>: Remote address (Host:Port) to connect to (required)
//       redial=<bool>: Keep redialing on lost connection (default: true)
//       tls=<string>: Name of TLS client config
//       cost=<float64>: Connection cost (weight) (default: 1.0)
//       allowedpeers=<[]string (may be repeated)>: Peer node IDs to allow via this connection

//    --udp-listener: Run a backend listener on a UDP port
//       bindaddr=<string>: Local address to bind to (default: 0.0.0.0)
//       port=<int>: Local UDP port to listen on (required)
//       cost=<float64>: Connection cost (weight) (default: 1.0)
//       nodecost=<JSON dict of string to float64>: Per-node costs
//       allowedpeers=<[]string (may be repeated)>: Peer node IDs to allow via this connection

//    --udp-peer: Make an outbound backend connection to a UDP peer
//       address=<string>: Host:Port to connect to (required)
//       redial=<bool>: Keep redialing on lost connection (default: true)
//       cost=<float64>: Connection cost (weight) (default: 1.0)
//       allowedpeers=<[]string (may be repeated)>: Peer node IDs to allow via this connection

//    --ws-listener: Run an http server that accepts websocket connections
//       bindaddr=<string>: Local address to bind to (default: 0.0.0.0)
//       port=<int>: Local TCP port to run http server on (required)
//       path=<string>: URI path to the websocket server (default: /)
//       tls=<string>: Name of TLS server config
//       cost=<float64>: Connection cost (weight) (default: 1.0)
//       nodecost=<JSON dict of string to float64>: Per-node costs
//       allowedpeers=<[]string (may be repeated)>: Peer node IDs to allow via this connection

//    --ws-peer: Connect outbound to a websocket peer
//       address=<string>: URL to connect to (required)
//       redial=<bool>: Keep redialing on lost connection (default: true)
//       extraheader=<string>: Sends extra HTTP header on initial connection
//       tls=<string>: Name of TLS client config
//       cost=<float64>: Connection cost (weight) (default: 1.0)
//       allowedpeers=<[]string (may be repeated)>: Peer node IDs to allow via this connection
