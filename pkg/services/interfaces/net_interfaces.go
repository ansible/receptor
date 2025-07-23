package netinterface

import (
	"net"
	"net/netip"
	"os"
	"syscall"
	"time"
)

type NetterUDP interface {
	ResolveUDPAddr(network string, address string) (*net.UDPAddr, error)
	ListenUDP(network string, laddr *net.UDPAddr) (UDPConnInterface, error)
	DialUDP(network string, laddr *net.UDPAddr, raddr *net.UDPAddr) (UDPConnInterface, error)
}

// UDPConnInterface abstracts the methods of net.UDPConn used in the proxy.
type UDPConnInterface interface {
	Close() error
	File() (f *os.File, err error)
	LocalAddr() net.Addr
	Read(b []byte) (int, error)
	ReadFrom(b []byte) (int, net.Addr, error)
	ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error)
	ReadFromUDPAddrPort(b []byte) (n int, addr netip.AddrPort, err error)
	ReadMsgUDP(b []byte, oob []byte) (n int, oobn int, flags int, addr *net.UDPAddr, err error)
	ReadMsgUDPAddrPort(b []byte, oob []byte) (n int, oobn int, flags int, addr netip.AddrPort, err error)
	RemoteAddr() net.Addr
	SetDeadline(t time.Time) error
	SetReadBuffer(bytes int) error
	SetReadDeadline(t time.Time) error
	SetWriteBuffer(bytes int) error
	SetWriteDeadline(t time.Time) error
	SyscallConn() (syscall.RawConn, error)
	Write(b []byte) (int, error)
	WriteMsgUDP(b []byte, oob []byte, addr *net.UDPAddr) (n int, oobn int, err error)
	WriteMsgUDPAddrPort(b []byte, oob []byte, addr netip.AddrPort) (n int, oobn int, err error)
	WriteTo(b []byte, addr net.Addr) (int, error)
	WriteToUDP(b []byte, addr *net.UDPAddr) (int, error)
	WriteToUDPAddrPort(b []byte, addr netip.AddrPort) (int, error)
}
