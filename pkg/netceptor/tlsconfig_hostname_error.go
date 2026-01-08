//go:build go1.23

package netceptor

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
)

// handleHostnameError handles a hostname error from the Certificate.Verify function. It
// was put in place temporarily to mitigate CVE-2025-61729 until the project is able to
// upgrade to a Golang version containing a fix.
//
// This function should be removed once the project can upgrade to a Golang version with
// a fix for the CVE.
func handleHostnameError(h x509.HostnameError) error {
	c := h.Certificate
	maxNamesIncluded := 100

	var valid strings.Builder
	if net.ParseIP(h.Host) != nil {
		// Trying to validate an IP
		if len(c.IPAddresses) == 0 {
			return errors.New("x509: cannot validate certificate for " + h.Host + " because it doesn't contain any IP SANs")
		}
		if len(c.IPAddresses) >= maxNamesIncluded {
			return fmt.Errorf("x509: certificate is valid for %d IP SANs, but none matched %s", len(c.IPAddresses), h.Host)
		}
		for _, san := range c.IPAddresses {
			if valid.Len() > 0 {
				valid.WriteString(", ")
			}
			valid.WriteString(san.String())
		}
	} else {
		if len(c.DNSNames) >= maxNamesIncluded {
			return fmt.Errorf("x509: certificate is valid for %d names, but none matched %s", len(c.DNSNames), h.Host)
		}
		valid.WriteString(strings.Join(c.DNSNames, ", "))
	}

	if valid.Len() == 0 {
		return errors.New("x509: certificate is not valid for any names, but wanted to match " + h.Host)
	}

	return errors.New("x509: certificate is valid for " + valid.String() + ", not " + h.Host)
}
