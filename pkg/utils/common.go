package utils

import (
	"crypto/x509"
	"fmt"
	"net"
	"strings"

	"github.com/ansible/receptor/pkg/logger"
)

func ParseReceptorNamesFromCert(cert *x509.Certificate, expectedHostname string, logger *logger.ReceptorLogger) (bool, []string, error) {
	var receptorNames []string
	receptorNames, err := ReceptorNames(cert.Extensions)
	if err != nil {
		logger.Error("RVF failed to get ReceptorNames: %s", err)

		return false, nil, err
	}
	found := false
	for _, receptorName := range receptorNames {
		if receptorName == expectedHostname {
			found = true

			break
		}
	}

	return found, receptorNames, nil
}

// AddressToHostPort splits an address(1.2.3.4:5000) into a Host(1.2.3.4) and a Port(5000). Enhances `net.SplitHostPort` to handle additional input formats and IPv6.
func AddressToHostPort(address string) (string, string, error) {
	if !strings.Contains(address, "[") && strings.Count(address, ":") > 2 {
		idx := strings.LastIndexByte(address, ':')
		if idx == -1 {
			return "", "", fmt.Errorf("malformed remote address: %v", address)
		}
		host := address[:idx]
		port := address[idx+1:]

		return host, port, nil
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", "", fmt.Errorf("%s is not a valid IP address + port", address)
	}

	return host, port, nil
}
