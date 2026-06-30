//go:build !darwin && !linux && !freebsd

package internal

import (
	"errors"
	"net"
)

// BoundDialer returns a dialer that pins outgoing TCP/UDP sockets to iface via
// SO_BINDTOIF. Platforms other than darwin/linux/freebsd are not supported by
// this implementation; the caller is expected to detect this at init time and
// fall back to the default dialer.
func BoundDialer(iface string) (*net.Dialer, error) {
	return nil, errors.New("BoundDialer: SO_BINDTOIF not implemented on this platform; unset USQUE_BIND_IFACE")
}