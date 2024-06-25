package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ansible/receptor/pkg/certificates"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	certInitBits          int
	certInitCommonName    string
	certInitNotAfter      string
	certInitNotBefore     string
	certInitOutCert       string
	certInitOutKey        string
	certMakeReqBits       int
	certMakeReqCommonName string
	certMakeReqDNSName    []string
	certMakeReqInKey      string
	certMakeReqIpaddress  []string
	certMakeReqNodeId     []string
	certMakeReqOutReq     string
	certMakeReqOutKey     string
	certSignReqRequest    string
	certSignReqCaCert     string
	certSignReqCaKey      string
	certSignReqOutCert    string
	certSignReqVerify     bool
	certSignReqNotAfter   string
	certSignReqNotBefore  string
)

func parseFlagSlice(flag *pflag.Flag) []string {
	strSlice := []string{}
	split := strings.Split(flag.Value.String(), ",")
	for index, val := range split {
		if index == 0 {
			v, ok := strings.CutPrefix(val, "[")
			if !ok {
				return nil
			}
			val = v
		}
		if index == len(split)-1 {
			v, ok := strings.CutSuffix(val, "]")
			if !ok {
				return nil
			}
			val = v
		}
		strSlice = append(strSlice, val)
	}
	return strSlice
}

var certInit = &cobra.Command{
	Use:   "cert-init",
	Short: "Initialize PKI CA",
	Long:  `Initialize PKI CA`,
	Run: func(cmd *cobra.Command, args []string) {
		bits, err := strconv.Atoi(cmd.Flag("bits").Value.String())
		if err != nil {
			fmt.Println(err)
		}
		initCAConfig := certificates.InitCAConfig{
			CommonName: certInitCommonName,
			Bits:       bits,
			NotBefore:  certInitNotBefore,
			NotAfter:   certInitNotAfter,
			OutCert:    certInitOutCert,
			OutKey:     certInitOutKey,
		}

		err = initCAConfig.Run()
		if err != nil {
			fmt.Println(err)
		}

		// viper.Set("InitCA", certificates.InitCAConfig{
		// 	CommonName: cmd.Flag("commonname").Value.String(),
		// 	Bits:       bits,
		// 	NotBefore:  cmd.Flag("notbefore").Value.String(),
		// 	NotAfter:   cmd.Flag("notafter").Value.String(),
		// 	OutCert:    cmd.Flag("outcert").Value.String(),
		// 	OutKey:     cmd.Flag("outkey").Value.String(),
		// })
	},
}

func addCertInitFlags() {
	certInit.Flags().IntVar(&certInitBits, "bits", 2048, "Bit length of the encryption keys of the certificate (required)")
	certInit.Flags().StringVar(&certInitCommonName, "commonname", "", "Bit length of the encryption keys of the certificate (required)")
	certInit.Flags().StringVar(&certInitNotAfter, "notafter", "", "Expiration (NotAfter) date/time, in RFC3339 format")
	certInit.Flags().StringVar(&certInitNotBefore, "notbefore", "", "Effective (NotBefore) date/time, in RFC3339 format")
	certInit.Flags().StringVar(&certInitOutCert, "outcert", "", "File to save the CA certificate to (required)")
	certInit.Flags().StringVar(&certInitOutKey, "outkey", "", "File to save the CA private key to (required)")
}

var certMakeReq = &cobra.Command{
	Use:   "cert-makereq",
	Short: "Create certificate request",
	Long:  `Create certificate request`,
	Run: func(cmd *cobra.Command, args []string) {
		bits, err := strconv.Atoi(cmd.Flag("bits").Value.String())
		if err != nil {
			fmt.Println(err)
		}

		dnsNames := parseFlagSlice(cmd.Flag("dnsname"))
		fmt.Println(dnsNames)
		ipAddresses := parseFlagSlice(cmd.Flag("ipaddress"))
		nodeIds := parseFlagSlice(cmd.Flag("nodeid"))

		makeReqConfig := certificates.MakeReqConfig{
			CommonName: certMakeReqCommonName,
			Bits:       bits,
			DNSName:    dnsNames,
			IPAddress:  ipAddresses,
			NodeID:     nodeIds,
			OutReq:     certMakeReqOutReq,
			InKey:      certMakeReqInKey,
			OutKey:     certMakeReqOutKey,
		}

		err = makeReqConfig.Prepare()
		if err != nil {
			fmt.Println(err)
		}

		err = makeReqConfig.Run()
		if err != nil {
			fmt.Println(err)
		}
		// viper.Set("MakeReq", certificates.MakeReqConfig{
		// 	CommonName: cmd.Flag("commonname").Value.String(),
		// 	Bits:       bits,
		// 	DNSName:    dnsNames,
		// 	IPAddress:  ipAddresses,
		// 	NodeID:     nodeIds,
		// 	OutReq:     cmd.Flag("outreq").Value.String(),
		// 	InKey:      cmd.Flag("inkey").Value.String(),
		// 	OutKey:     cmd.Flag("outkey").Value.String(),
		// })
	},
}

func addCertMakeReqFlags() {
	certMakeReq.Flags().IntVar(&certMakeReqBits, "bits", 2048, "Bit length of the encryption keys of the certificate (required)")
	certMakeReq.Flags().StringVar(&certMakeReqCommonName, "commonname", "", "Common name to assign to the certificate (required)")
	certMakeReq.Flags().StringSlice("dnsname", certMakeReqDNSName, "DNS names to add to the certificate")
	certMakeReq.Flags().StringVar(&certMakeReqInKey, "inkey", "", "Private key to use for the request")
	certMakeReq.Flags().StringSlice("ipaddress", certMakeReqIpaddress, "IP addresses to add to the certificate")
	certMakeReq.Flags().StringSlice("nodeid", certMakeReqNodeId, "Receptor node IDs to add to the certificate")
	certMakeReq.Flags().StringVar(&certMakeReqOutReq, "outreq", "", "File to save the certificate request to (required)")
	certMakeReq.Flags().StringVar(&certMakeReqOutKey, "outkey", "", "File to save the private key to (new key will be generated)")
}

var certSignReq = &cobra.Command{
	Use:   "cert-signreq",
	Short: "Sign request and produce certificate",
	Long:  `Sign request and produce certificate`,
	Run: func(cmd *cobra.Command, args []string) {

		verify, err := strconv.ParseBool(cmd.Flag("verify").Value.String())
		if err != nil {
			fmt.Println(err)
		}

		signReqConfig := certificates.SignReqConfig{
			Req:       certSignReqRequest,
			CACert:    certSignReqCaCert,
			CAKey:     certSignReqCaKey,
			NotBefore: certSignReqNotBefore,
			NotAfter:  certSignReqNotAfter,
			OutCert:   certSignReqOutCert,
			Verify:    verify,
		}

		err = signReqConfig.Run()
		if err != nil {
			fmt.Println(err)
		}

		// viper.Set("", certificates.SignReqConfig{
		// 	Req:       cmd.Flag("req").Value.String(),
		// 	CACert:    cmd.Flag("cacert").Value.String(),
		// 	CAKey:     cmd.Flag("cakey").Value.String(),
		// 	NotBefore: cmd.Flag("notbefore").Value.String(),
		// 	NotAfter:  cmd.Flag("notafter").Value.String(),
		// 	OutCert:   cmd.Flag("outcert").Value.String(),
		// 	Verify:    verify,
		// })
	},
}

func addCertSignReqFlags() {
	certMakeReq.Flags().StringVar(&certSignReqRequest, "req", "", "Certificate Request PEM filename (required)")
	certMakeReq.Flags().StringVar(&certSignReqCaCert, "cacert", "", "CA certificate PEM filename (required)")
	certMakeReq.Flags().StringVar(&certSignReqCaKey, "cakey", "", "CA private key PEM filename (required)")
	certMakeReq.Flags().StringVar(&certSignReqNotAfter, "notafter", "", "Expiration (NotAfter) date/time, in RFC3339 format")
	certMakeReq.Flags().StringVar(&certSignReqNotBefore, "notbefore", "", "Effective (NotBefore) date/time, in RFC3339 format")
	certMakeReq.Flags().StringVar(&certSignReqOutCert, "outcert", "", "File to save the signed certificate to (required)")
	certMakeReq.Flags().BoolVar(&certSignReqVerify, "verify", false, "If true, do not prompt the user for verification")
}
