// Copyright (c) 2014 Maurice Nonnekes <maurice@codeninja.nl>
// All rights reserved.

// Package socketpair implements a simple interface to create a socket pair.
package socketpair

import (
	"errors"
	"net"
	"os"
	"syscall"
)

// New creates a socketpair of the specified network type.
//
// Known networks are "unix" and "unixgram".
func New(network string) (net.Conn, net.Conn, error) {
	var spDomain, spType int

	switch network {
	case "unix":
		spDomain, spType = syscall.AF_LOCAL, syscall.SOCK_STREAM
	case "unixgram":
		spDomain, spType = syscall.AF_LOCAL, syscall.SOCK_DGRAM
	default:
		return nil, nil, errors.New("unknown network " + network)
	}

	fds, err := syscall.Socketpair(spDomain, spType, 0)
	if err != nil {
		return nil, nil, err
	}

	fd1 := os.NewFile(uintptr(fds[0]), "fd1")
	defer fd1.Close()

	fd2 := os.NewFile(uintptr(fds[1]), "fd2")
	defer fd2.Close()

	sock1, err := net.FileConn(fd1)
	if err != nil {
		return nil, nil, err
	}

	sock2, err := net.FileConn(fd2)
	if err != nil {
		sock1.Close()
		return nil, nil, err
	}

	return sock1, sock2, nil
}
