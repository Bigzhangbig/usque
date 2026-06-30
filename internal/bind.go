//go:build darwin || linux || freebsd

package internal

import (
	"net"
	"syscall"
)

// BoundDialer returns a *net.Dialer whose Control callback binds every TCP/UDP
// socket to iface via SO_BINDTOIF (IP_BOUND_IF). macOS, Linux, and the BSDs
// accept the interface index as the option value.
func BoundDialer(iface string) (*net.Dialer, error) {
	netIface, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, err
	}

	idx := netIface.Index
	return &net.Dialer{
		Control: func(network, address string, c syscall.RawConn) error {
			var sockErr error
			err := c.Control(func(fd uintptr) {
				// IP_BOUND_IF = 25 on both Linux and macOS.
				sockErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, 25, idx)
			})
			if err != nil {
				return err
			}
			return sockErr
		},
	}, nil
}