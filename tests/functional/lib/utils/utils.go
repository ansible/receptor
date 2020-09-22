package utils

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var udpPortMutex sync.Mutex
var udpPortPool []int

var tcpPortMutex sync.Mutex
var tcpPortPool []int

func init() {
	udpPortMutex.Lock()
	defer udpPortMutex.Unlock()
	udpPortPool, _ = makeRange(10000, 65000, 1)
	tcpPortMutex.Lock()
	defer tcpPortMutex.Unlock()
	tcpPortPool, _ = makeRange(10000, 65000, 1)
}

func makeRange(start, stop, step int) ([]int, error) {
	out := []int{}
	if step > 0 && start < stop {
		for ; start < stop; start += step {
			out = append(out, start)
		}
	} else if step < 0 && start > stop {
		for ; start > stop; start += step {
			out = append(out, start)
		}
	} else {
		return nil, errors.New("Unable to make range")
	}
	return out, nil
}

// ReserveTCPPort generates an unused TCP Port, When you are done using this
// port, you must call FreeTCPPort
// There's a race condition here where the port we grab *could* later be
// grabbed by another process/thread before we use it, if you rely on this you
// should handle a case where the port given is in use before you are able to
// open it
func ReserveTCPPort() int {
	tcpPortMutex.Lock()
	defer tcpPortMutex.Unlock()

	for {
		portNum := tcpPortPool[len(tcpPortPool)-1]
		tcpPortPool = tcpPortPool[:len(tcpPortPool)-1]
		portStr := strconv.Itoa(portNum)
		tcpPort, err := net.Listen("tcp", ":"+portStr)
		if err == nil {
			tcpPort.Close()
			tcpPortPool = tcpPortPool[:len(tcpPortPool)-1]
			return portNum
		}
		// If we havent reserved this port but it's taken, prepend it to
		// our list so eventually we can check if it's still in use, if we
		// take it out of the list permanently we can never check it again
		tcpPortPool = append([]int{portNum}, tcpPortPool...)
	}
}

// FreeTCPPort puts a port back into the pool such that it can be allocated
// later
func FreeTCPPort(portNum int) {
	tcpPortMutex.Lock()
	defer tcpPortMutex.Unlock()

	tcpPortPool = append(tcpPortPool, portNum)
}

// ReserveUDPPort generates a likely unused UDP Port
// There's a race condition here where the port we grab *could* later be
// grabbed by another process/thread before we use it, if you rely on this you
// should handle a case where the port given is in use before you are able to
// open it
func ReserveUDPPort() int {
	udpPortMutex.Lock()
	defer udpPortMutex.Unlock()

	for {
		portNum := udpPortPool[len(udpPortPool)-1]
		udpPortPool = udpPortPool[:len(udpPortPool)-1]
		portStr := strconv.Itoa(portNum)
		//udpPort, err := net.Listen("udp", ":"+portStr)
		udpAddr, err := net.ResolveUDPAddr("udp", ":"+portStr)
		if err != nil {
			panic("err")
		}
		udpConn, err := net.ListenUDP("udp", udpAddr)

		if err == nil {
			udpConn.Close()
			udpPortPool = udpPortPool[:len(udpPortPool)-1]
			return portNum
		}
		// If we havent reserved this port but it's taken, prepend it to
		// our list so eventually we can check if it's still in use, if we
		// take it out of the list permanently we can never check it again
		udpPortPool = append([]int{portNum}, udpPortPool...)
	}
}

// FreeUDPPort puts a port back into the pool such that it can be allocated
// later
func FreeUDPPort(portNum int) {
	udpPortMutex.Lock()
	defer udpPortMutex.Unlock()

	udpPortPool = append(udpPortPool, portNum)
}

// GenerateCert generates a private and public key for testing in the directory
// specified
func GenerateCert(dir, name string) (keyPath, certPath string, e error) {
	KeyPath := filepath.Join(dir, name+".key")
	CrtPath := filepath.Join(dir, name+".crt")
	// Create our private key
	cmd := exec.Command("openssl", "genrsa", "-out", KeyPath, "1024")
	err := cmd.Run()
	if err != nil {
		return "", "", err
	}
	// Create our certificate
	cmd = exec.Command("openssl", "req", "-x509", "-new", "-nodes", "-key", KeyPath, "-subj", "/C=/ST=/L=/O=Receptor Testing/OU=/CN=localhost", "-sha256", "-out", CrtPath)
	err = cmd.Run()
	if err != nil {
		return "", "", err
	}
	return KeyPath, CrtPath, nil
}

// GenerateCertWithCA generates a private and public key for testing in the directory
// specified using the ca specified
func GenerateCertWithCA(dir, name, caKeyPath, caCrtPath string) (keyPath, certPath string, e error) {
	KeyPath := filepath.Join(dir, name+".key")
	CrtPath := filepath.Join(dir, name+".crt")
	CSRPath := filepath.Join(dir, name+".csa")
	// Create our private key
	cmd := exec.Command("openssl", "genrsa", "-out", KeyPath, "1024")
	err := cmd.Run()
	if err != nil {
		return "", "", err
	}

	// Create our certificate request
	cmd = exec.Command("openssl", "req", "-new", "-sha256", "-key", KeyPath, "-subj", "/C=/ST=/L=/O=Receptor Testing/OU=/CN=localhost", "-out", CSRPath)
	err = cmd.Run()
	if err != nil {
		return "", "", err
	}

	// Create our certificate using the CA
	cmd = exec.Command("openssl", "x509", "-req", "-in", CSRPath, "-CA", caCrtPath, "-CAkey", caKeyPath, "-CAcreateserial", "-out", CrtPath, "-sha256")
	err = cmd.Run()
	if err != nil {
		return "", "", err
	}
	return KeyPath, CrtPath, nil
}

// CheckUntilTimeout Polls the check function until the context expires, in
// which case it returns false
func CheckUntilTimeout(ctx context.Context, interval time.Duration, check func() bool) bool {
	for ready := check(); !ready; ready = check() {
		if ctx.Err() != nil {
			return false
		}
		time.Sleep(interval)
	}
	return true
}

// CheckUntilTimeoutWithErr does the same as CheckUntilTimeout but requires the
// check function returns (bool, error), and will return an error immediately
// if the check function returns an error
func CheckUntilTimeoutWithErr(ctx context.Context, interval time.Duration, check func() (bool, error)) (bool, error) {
	for ready, err := check(); !ready; ready, err = check() {
		if err != nil {
			return false, err
		}
		if ctx.Err() != nil {
			return false, nil
		}
		time.Sleep(interval)
	}
	return true, nil
}
