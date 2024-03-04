package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	bindAddr string
	// should port be a string?
	port                   int
	tlsConfig              string
	connectionCost         float64
	nodeCost               map[string]int
	allowedPeers           []string
	remoteAddress          string
	keepRedial             bool
	tlsClientConfig        string
	localAddress           string
	localPort              int
	udpPeerAddress         string
	udpPeerRedial          bool
	udpPeerCost            float64
	udpPeerAllowedPeers    []string
	wsListenerBindAddr     string
	wsListenerPort         int
	wsListenerPath         string
	wsListenerTLS          string
	wsListenerCost         float64
	wsListenerAllowedPeers []string
	wsPeerAddress          string
	wsPeerRedial           bool
	wsPeerExtraHeader      string
	wsPeerTLS              string
	wsPeerCost             float64
	wsPeerAllowedPeers     []string
)

var tcpListener = &cobra.Command{
	Use:   "tcp-listener",
	Short: "Run a backend listener on a TCP port",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("tcpListener called with port %v\n", port)
	},
}

var tcpPeer = &cobra.Command{
	Use:   "tcp-peer",
	Short: "Make an outbound backend connection to a TCP peer",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var udpListener = &cobra.Command{
	Use:   "udp-listener",
	Short: "Run a backend listener on a UDP port",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var udpPeer = &cobra.Command{
	Use:   "udp-peer",
	Short: "Make an outbound backend connection to a UDP peer",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var wsListener = &cobra.Command{
	Use:   "ws-listener",
	Short: "Run an http server that accepts websocket connections",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var wsPeer = &cobra.Command{
	Use:   "ws-peer",
	Short: "Connect outbound to a websocket peer",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

func init() {

	tcpListener.Flags().StringVar(&bindAddr, "bindaddr", "0.0.0.0", "Local address to bind to (default: 0.0.0.0)")
	tcpListener.Flags().IntVar(&port, "port", 0, "Local TCP port to listen on (required)")
	tcpListener.MarkFlagRequired("port")
	tcpListener.Flags().StringVar(&tlsConfig, "tls", "", "Name of TLS server config")
	tcpListener.Flags().Float64Var(&connectionCost, "cost", 1.0, "Connection cost (weight) (default: 1.0)")
	tcpListener.Flags().StringToIntVarP(&nodeCost, "nodecost", "", nil, "Per-node costs")
	tcpListener.Flags().StringSliceVar(&allowedPeers, "allowedpeers", nil, "Peer node IDs to allow via this connection")

	tcpPeer.Flags().StringVar(&remoteAddress, "address", "", "Remote address (Host:Port) to connect to (required)")
	tcpPeer.MarkFlagRequired("address")
	tcpPeer.Flags().BoolVar(&keepRedial, "redial", true, "Keep redialing on lost connection (default: true)")
	tcpPeer.Flags().StringVar(&tlsClientConfig, "tls", "", "Name of TLS client config")
	tcpPeer.Flags().Float64Var(&connectionCost, "cost", 1.0, "Connection cost (weight) (default: 1.0)")
	tcpPeer.Flags().StringSliceVar(&allowedPeers, "allowedpeers", nil, "Peer node IDs to allow via this connection")

	udpListener.Flags().StringVar(&localAddress, "bindaddr", "0.0.0.0", "Local address to bind to (default: 0.0.0.0)")
	udpListener.Flags().IntVar(&localPort, "port", 0, "Local UDP port to listen on (required)")
	udpListener.MarkFlagRequired("port")
	udpListener.Flags().Float64Var(&connectionCost, "cost", 1.0, "Connection cost (weight) (default: 1.0)")
	udpListener.Flags().StringToIntVarP(&nodeCost, "nodecost", "", nil, "Per-node costs")
	udpListener.Flags().StringSliceVar(&allowedPeers, "allowedpeers", nil, "Peer node IDs to allow via this connection")

	udpPeer.Flags().StringVar(&udpPeerAddress, "address", "", "Host:Port to connect to (required)")
	udpPeer.MarkFlagRequired("address")
	udpPeer.Flags().BoolVar(&udpPeerRedial, "redial", true, "Keep redialing on lost connection (default: true)")
	udpPeer.Flags().Float64Var(&udpPeerCost, "cost", 1.0, "Connection cost (weight) (default: 1.0)")
	udpPeer.Flags().StringSliceVar(&udpPeerAllowedPeers, "allowedpeers", nil, "Peer node IDs to allow via this connection")

	wsListener.Flags().StringVar(&wsListenerBindAddr, "bindaddr", "0.0.0.0", "Local address to bind to (default: 0.0.0.0)")
	wsListener.Flags().IntVar(&wsListenerPort, "port", 0, "Local TCP port to run http server on (required)")
	wsListener.MarkFlagRequired("port")
	wsListener.Flags().StringVar(&wsListenerPath, "path", "/", "URI path to the websocket server (default: /)")
	wsListener.Flags().StringVar(&wsListenerTLS, "tls", "", "Name of TLS server config")
	wsListener.Flags().Float64Var(&wsListenerCost, "cost", 1.0, "Connection cost (weight) (default: 1.0)")
	wsListener.Flags().StringToIntVarP(&nodeCost, "nodecost", "", nil, "Per-node costs")
	wsListener.Flags().StringSliceVar(&wsListenerAllowedPeers, "allowedpeers", nil, "Peer node IDs to allow via this connection")

	wsPeer.Flags().StringVar(&wsPeerAddress, "address", "", "URL to connect to (required)")
	wsPeer.MarkFlagRequired("address")
	wsPeer.Flags().BoolVar(&wsPeerRedial, "redial", true, "Keep redialing on lost connection (default: true)")
	wsPeer.Flags().StringVar(&wsPeerExtraHeader, "extraheader", "", "Sends extra HTTP header on initial connection")
	wsPeer.Flags().StringVar(&wsPeerTLS, "tls", "", "Name of TLS client config")
	wsPeer.Flags().Float64Var(&wsPeerCost, "cost", 1.0, "Connection cost (weight) (default: 1.0)")
	wsPeer.Flags().StringSliceVar(&wsPeerAllowedPeers, "allowedpeers", nil, "Peer node IDs to allow via this connection")

	rootCmd.AddCommand(tcpListener, tcpPeer, udpListener, udpPeer, wsListener, wsPeer)

}
