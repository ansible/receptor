package utils

import "net"

type NetListener interface {
	net.Listener
}

type NetConn interface {
	net.Conn
}

type NetAddr interface {
	net.Addr
}
