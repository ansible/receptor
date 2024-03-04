package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	certInitCommonName    string
	certInitBits          int
	certInitNotBefore     string
	certInitNotAfter      string
	certInitOutCert       string
	certInitOutKey        string
	certMakeReqCommonName string
	certMakeReqBits       int
	certMakeReqDNSName    []string
	certMakeReqIPAddress  []string
	certMakeReqNodeID     []string
	certMakeReqOutReq     string
	certMakeReqInKey      string
	certMakeReqOutKey     string
	certSignReqReq        string
	certSignReqCACert     string
	certSignReqCAKey      string
	certSignReqNotBefore  string
	certSignReqNotAfter   string
	certSignReqOutCert    string
	certSignReqVerify     bool
)

var certInit = &cobra.Command{
	Use:   "cert-init",
	Short: "Initialize PKI CA",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var certMakeReq = &cobra.Command{
	Use:   "cert-makereq",
	Short: "Create certificate request",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var certSignReq = &cobra.Command{
	Use:   "cert-signreq",
	Short: "Sign request and produce certificate",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

func init() {
	certInit.Flags().StringVar(&certInitCommonName, "commonname", "", "Common name to assign to the certificate (required)")
	certInit.MarkFlagRequired("commonname")
	certInit.Flags().IntVar(&certInitBits, "bits", 0, "Bit length of the encryption keys of the certificate (required)")
	certInit.MarkFlagRequired("bits")
	certInit.Flags().StringVar(&certInitNotBefore, "notbefore", "", "Effective (NotBefore) date/time, in RFC3339 format")
	certInit.Flags().StringVar(&certInitNotAfter, "notafter", "", "Expiration (NotAfter) date/time, in RFC3339 format")
	certInit.Flags().StringVar(&certInitOutCert, "outcert", "", "File to save the CA certificate to (required)")
	certInit.MarkFlagRequired("outcert")
	certInit.Flags().StringVar(&certInitOutKey, "outkey", "", "File to save the CA private key to (required)")
	certInit.MarkFlagRequired("outkey")

	certMakeReq.Flags().StringVar(&certMakeReqCommonName, "commonname", "", "Common name to assign to the certificate (required)")
	certMakeReq.MarkFlagRequired("commonname")
	certMakeReq.Flags().IntVar(&certMakeReqBits, "bits", 0, "Bit length of the encryption keys of the certificate")
	certMakeReq.Flags().StringSliceVar(&certMakeReqDNSName, "dnsname", nil, "DNS names to add to the certificate")
	certMakeReq.Flags().StringSliceVar(&certMakeReqIPAddress, "ipaddress", nil, "IP addresses to add to the certificate")
	certMakeReq.Flags().StringSliceVar(&certMakeReqNodeID, "nodeid", nil, "Receptor node IDs to add to the certificate")
	certMakeReq.Flags().StringVar(&certMakeReqOutReq, "outreq", "", "File to save the certificate request to (required)")
	certMakeReq.MarkFlagRequired("outreq")
	certMakeReq.Flags().StringVar(&certMakeReqInKey, "inkey", "", "Private key to use for the request")
	certMakeReq.Flags().StringVar(&certMakeReqOutKey, "outkey", "", "File to save the private key to (new key will be generated)")

	certSignReq.Flags().StringVar(&certSignReqReq, "req", "", "Certificate Request PEM filename (required)")
	certSignReq.MarkFlagRequired("req")
	certSignReq.Flags().StringVar(&certSignReqCACert, "cacert", "", "CA certificate PEM filename (required)")
	certSignReq.MarkFlagRequired("cacert")
	certSignReq.Flags().StringVar(&certSignReqCAKey, "cakey", "", "CA private key PEM filename (required)")
	certSignReq.MarkFlagRequired("cakey")
	certSignReq.Flags().StringVar(&certSignReqNotBefore, "notbefore", "", "Effective (NotBefore) date/time, in RFC3339 format")
	certSignReq.Flags().StringVar(&certSignReqNotAfter, "notafter", "", "Expiration (NotAfter) date/time, in RFC3339 format")
	certSignReq.Flags().StringVar(&certSignReqOutCert, "outcert", "", "File to save the signed certificate to (required)")
	certSignReq.MarkFlagRequired("outcert")
	certSignReq.Flags().BoolVar(&certSignReqVerify, "verify", false, "If true, do not prompt the user for verification (default: False)")

	rootCmd.AddCommand(certInit, certMakeReq, certSignReq)

}
