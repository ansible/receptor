package utils

import (
	"crypto/x509"

	"github.com/ansible/receptor/pkg/logger"
)

func ParseReceptorNamesFromCert(cert *x509.Certificate, expectedHostname string) (bool, []string, error) {
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
