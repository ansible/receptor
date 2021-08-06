package main

import (
	"time"

	"github.com/project-receptor/receptor/pkg/certificates"
	"github.com/spf13/cobra"
)

func signReqCmd() *cobra.Command {
	var notBefore, notAfter string
	var caCrt, caKey, req, certOut string
	var verify bool

	cmd := &cobra.Command{
		Use:   "sign",
		Short: "Sign a certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &certificates.CertOptions{}
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

			return certificates.SignReq(opts, caCrt, caKey, req, certOut, verify)
		},
	}

	cmd.Flags().StringVar(&notBefore, "not-before", "", "Effective (NotBefore) date/time, in RFC3339 format")
	cmd.Flags().StringVar(&notAfter, "not-after", "", "Expiration (NotAfter) date/time, in RFC3339 format")

	cmd.Flags().StringVar(&req, "req", "", "Certificate Request PEM filename")
	cmd.MarkFlagRequired("req")
	cmd.MarkFlagFilename("req")

	cmd.Flags().StringVar(&caCrt, "ca-crt", "", "CA certificate PEM filename")
	cmd.MarkFlagRequired("ca-cert")
	cmd.MarkFlagFilename("ca-cert")

	cmd.Flags().StringVar(&caKey, "ca-key", "", "CA private key PEM filename")
	cmd.MarkFlagRequired("ca-key")
	cmd.MarkFlagFilename("ca-key")

	cmd.Flags().StringVar(&certOut, "cert-out", "", "File to save the signed certificate to")
	cmd.MarkFlagRequired("cert-out")
	cmd.MarkFlagFilename("cert-out")

	cmd.Flags().BoolVar(&verify, "verify", false, "If true, do not prompt the user for verification")

	return cmd
}
