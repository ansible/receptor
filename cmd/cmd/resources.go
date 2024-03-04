package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	serverName                   string
	serverCert                   string
	serverKey                    string
	serverRequireclientcert      bool
	serverClientcas              string
	serverPinnedclientcert       []string
	serverSkipreceptornamescheck bool
	serverMintls13               bool
	clientName                   string
	clientCert                   string
	clientKey                    string
	clientRootcas                string
	clientInsecureskipverify     bool
	clientPinnedservercert       []string
	clientSkipreceptornamescheck bool
	clientMintls13               bool
)

// resourcesCmd represents the resources command
var tlsServer = &cobra.Command{
	Use:   "tls-server",
	Short: "Define a TLS server configuration",
	Long:  ``,
	Args:  cobra.MinimumNArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var tlsClient = &cobra.Command{
	Use:   "tls-client",
	Short: "Define a TLS client configuration",
	Long:  ``,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

func init() {

	tlsServer.Flags().StringVarP(&serverName, "name", "n", "", "Name of this TLS server configuration (required)")
	tlsServer.MarkFlagRequired("name")

	tlsServer.Flags().StringVarP(&serverCert, "cert", "c", "", "Server certificate filename (required)")
	tlsServer.MarkFlagRequired("cert")

	tlsServer.Flags().StringVarP(&serverKey, "key", "k", "", "Server private key filename (required)")
	tlsServer.MarkFlagRequired("key")

	tlsServer.Flags().BoolVarP(&serverRequireclientcert, "requireclientcert", "", false, "Require client certificates (default: false)")
	tlsServer.Flags().StringVarP(&serverClientcas, "clientcas", "", "", "Filename of CA bundle to verify client certs with")
	tlsServer.Flags().StringSliceVarP(&serverPinnedclientcert, "pinnedclientcert", "", nil, "Pinned fingerprint of required client certificate")
	tlsServer.Flags().BoolVarP(&serverSkipreceptornamescheck, "skipreceptornamescheck", "", false, "Skip verifying ReceptorNames OIDs in certificate at startup (default: false)")
	tlsServer.Flags().BoolVarP(&serverMintls13, "mintls13", "", false, "Set minimum TLS version to 1.3. Otherwise the minimum is 1.2 (default: false)")

	tlsClient.Flags().StringVarP(&clientName, "name", "n", "", "Name of this TLS client configuration (required)")
	tlsClient.MarkFlagRequired("name")

	tlsClient.Flags().StringVarP(&clientCert, "cert", "c", "", "Client certificate filename")
	tlsClient.Flags().StringVarP(&clientKey, "key", "k", "", "Client private key filename")
	tlsClient.Flags().StringVarP(&clientRootcas, "rootcas", "", "", "Root CA bundle to use instead of system trust")
	tlsClient.Flags().BoolVarP(&clientInsecureskipverify, "insecureskipverify", "", false, "Accept any server cert (default: false)")
	tlsClient.Flags().StringSliceVarP(&clientPinnedservercert, "pinnedservercert", "", nil, "<[]string (may be repeated)>: Pinned fingerprint of required server certificate")
	tlsClient.Flags().BoolVarP(&clientSkipreceptornamescheck, "skipreceptornamescheck", "", false, "if true, skip verifying ReceptorNames OIDs in certificate at startup")
	tlsClient.Flags().BoolVarP(&clientMintls13, "mintls13", "", false, "Set minimum TLS version to 1.3. Otherwise the minimum is 1.2 (default: false)")

	rootCmd.AddCommand(tlsServer)
	rootCmd.AddCommand(tlsClient)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// resourcesCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// resourcesCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Commands that configure resources used by other commands:

//    --tls-server: Define a TLS server configuration
//       name=<string>: Name of this TLS server configuration (required)
//       cert=<string>: Server certificate filename (required)
//       key=<string>: Server private key filename (required)
//       requireclientcert=<bool>: Require client certificates (default: false)
//       clientcas=<string>: Filename of CA bundle to verify client certs with
//       pinnedclientcert=<[]string (may be repeated)>: Pinned fingerprint of required client certificate
//       skipreceptornamescheck=<bool>: Skip verifying ReceptorNames OIDs in certificate at startup (default: false)
//       mintls13=<bool>: Set minimum TLS version to 1.3. Otherwise the minimum is 1.2 (default: false)

//    --tls-client: Define a TLS client configuration
//       name=<string>: Name of this TLS client configuration (required)
//       cert=<string>: Client certificate filename
//       key=<string>: Client private key filename
//       rootcas=<string>: Root CA bundle to use instead of system trust
//       insecureskipverify=<bool>: Accept any server cert (default: false)
//       pinnedservercert=<[]string (may be repeated)>: Pinned fingerprint of required server certificate
//       skipreceptornamescheck=<bool>: if true, skip verifying ReceptorNames OIDs in certificate at startup
//       mintls13=<bool>: Set minimum TLS version to 1.3. Otherwise the minimum is 1.2 (default: false)
