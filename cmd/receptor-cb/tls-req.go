package main

import (
	"fmt"
	"net"

	"github.com/project-receptor/receptor/pkg/certificates"
	"github.com/spf13/cobra"
)

func makeReqCmd() *cobra.Command {
	var cn string
	var rsaBits int
	var dnsNames []string
	var ipAddresses []string
	var nodeIDs []string
	var keyIn, keyOut, reqOut string

	cmd := &cobra.Command{
		Use:   "req",
		Short: "Generate a certificate sign request",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &certificates.CertOptions{
				CommonName: cn,
				Bits:       rsaBits,
			}
			opts.DNSNames = dnsNames
			opts.NodeIDs = nodeIDs
			for _, ipstr := range ipAddresses {
				ip := net.ParseIP(ipstr)
				if ip == nil {
					return fmt.Errorf("invalid IP address: %s", ipstr)
				}
				if opts.IPAddresses == nil {
					opts.IPAddresses = make([]net.IP, 0)
				}
				opts.IPAddresses = append(opts.IPAddresses, ip)
			}

			return certificates.MakeReq(opts, keyIn, keyOut, reqOut)
		},
	}

	cmd.Flags().StringVar(&cn, "cn", "", "Common name to assign to the certificate")
	cmd.MarkFlagRequired("cn")

	cmd.Flags().IntVar(&rsaBits, "rsa-bits", 0, "Bit length of the encryption keys of the certificate")

	cmd.Flags().StringSliceVar(&dnsNames, "dns-name", []string{}, "DNS names to add to the certificate")

	cmd.Flags().StringSliceVar(&ipAddresses, "ip-address", []string{}, "IP addresses to add to the certificate")

	cmd.Flags().StringSliceVar(&nodeIDs, "node-id", []string{}, "Receptor node IDs to add to the certificate")

	cmd.Flags().StringVar(&keyOut, "key-out", "", "File to save the private key to (new key will be generated)")
	cmd.MarkFlagFilename("key-out")

	cmd.Flags().StringVar(&keyIn, "key-in", "", "Private key to use for the request")
	cmd.MarkFlagFilename("key-in")

	cmd.Flags().StringVar(&reqOut, "req-out", "", "File to save the certificate request to")
	cmd.MarkFlagRequired("req-out")
	cmd.MarkFlagFilename("req-out")

	return cmd
}
