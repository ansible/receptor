package utils

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/randstr"
)

// A generated host ID will be node-<8 char string> optionally with @<host identifier> where <host identifier will be either a hostname or an IP.
func GenerateHostID() string {
	var suffix string
	spacer := "@"

	hostname, err := os.Hostname()
	// If we got a hostname and its not like 'localhost' we will use that hostname as our suffix.
	if err == nil {
		lchost := strings.ToLower(hostname)
		if lchost != "localhost" && !strings.HasPrefix(lchost, "localhost.") {
			suffix = fmt.Sprintf("%s%s", spacer, hostname)
		}
	}
	// If our suffix is still empty lets try and get an IP for this machine
	if suffix == "" {
		// We didn't get a hostname as a suffix so lets see if we can find an IP
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, address := range addrs {
				// check the address type and if it is not a loopback we will just use the first one
				if ip, ok := address.(*net.IPNet); ok {
					if ip.IP.IsLoopback() {
						continue
					}
					suffix = fmt.Sprintf("%s%s", spacer, ip.IP.String())

					break
				}
			}
		}
	}

	generatedName := randstr.RandomStringWithPrefixAndSuffix("node-", 8, suffix)
	logger.Warning("node id is not set in the serve config generated an id of %s", generatedName)

	return generatedName
}
