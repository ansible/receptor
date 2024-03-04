package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// certCmd represents the cert command
var certCmd = &cobra.Command{
	Use:   "cert",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("cert called")
	},
}

func init() {
	rootCmd.AddCommand(certCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// certCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// certCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Commands to generate certificates and run a certificate authority

//    --cert-init: Initialize PKI CA
//       commonname=<string>: Common name to assign to the certificate (required)
//       bits=<int>: Bit length of the encryption keys of the certificate (required)
//       notbefore=<string>: Effective (NotBefore) date/time, in RFC3339 format
//       notafter=<string>: Expiration (NotAfter) date/time, in RFC3339 format
//       outcert=<string>: File to save the CA certificate to (required)
//       outkey=<string>: File to save the CA private key to (required)

//    --cert-makereq: Create certificate request
//       commonname=<string>: Common name to assign to the certificate (required)
//       bits=<int>: Bit length of the encryption keys of the certificate
//       dnsname=<[]string (may be repeated)>: DNS names to add to the certificate
//       ipaddress=<[]string (may be repeated)>: IP addresses to add to the certificate
//       nodeid=<[]string (may be repeated)>: Receptor node IDs to add to the certificate
//       outreq=<string>: File to save the certificate request to (required)
//       inkey=<string>: Private key to use for the request
//       outkey=<string>: File to save the private key to (new key will be generated)

//    --cert-signreq: Sign request and produce certificate
//       req=<string>: Certificate Request PEM filename (required)
//       cacert=<string>: CA certificate PEM filename (required)
//       cakey=<string>: CA private key PEM filename (required)
//       notbefore=<string>: Effective (NotBefore) date/time, in RFC3339 format
//       notafter=<string>: Expiration (NotAfter) date/time, in RFC3339 format
//       outcert=<string>: File to save the signed certificate to (required)
//       verify=<bool>: If true, do not prompt the user for verification (default: False)
