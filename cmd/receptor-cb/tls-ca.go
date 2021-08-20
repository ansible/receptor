package main

import (
	"time"

	"github.com/ansible/receptor/pkg/certificates"
	"github.com/spf13/cobra"
)

func initCaCmd() *cobra.Command {
	var cn string
	var rsaBits int
	var notBefore, notAfter string
	var certOut, keyOut string
	cmd := &cobra.Command{
		Use:   "ca",
		Short: "Generate a CA key and certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &certificates.CertOptions{
				CommonName: cn,
				Bits:       rsaBits,
			}

			if notBefore != "" {
				t, err := time.Parse(time.RFC3339, notBefore)
				if err != nil {
					return err
				}
				opts.NotBefore = t
			}
			if notAfter != "" {
				t, err := time.Parse(time.RFC3339, notAfter)
				if err != nil {
					return err
				}
				opts.NotAfter = t
			}

			return certificates.InitCA(opts, certOut, keyOut)
		},
	}

	cmd.Flags().StringVar(&cn, "cn", "", "Common name to assign to the certificate")
	cmd.MarkFlagRequired("cn")

	cmd.Flags().IntVar(&rsaBits, "rsa-bits", -1, "Bit length of the encryption keys of the certificate")
	cmd.MarkFlagRequired("rsa-bits")

	cmd.Flags().StringVar(&notBefore, "not-before", "", "Effective (NotBefore) date/time, in RFC3339 format")
	cmd.Flags().StringVar(&notAfter, "not-after", "", "Expiration (NotAfter) date/time, in RFC3339 format")

	cmd.Flags().StringVar(&certOut, "cert-out", "", "File to save the CA certificate to")
	cmd.MarkFlagRequired("cert-out")
	cmd.MarkFlagFilename("cert-out")

	cmd.Flags().StringVar(&keyOut, "key-out", "", "File to save the CA private key to")
	cmd.MarkFlagRequired("key-out")
	cmd.MarkFlagFilename("key-out")

	return cmd
}
