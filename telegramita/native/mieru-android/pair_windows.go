//go:build windows

package main

import "net"

// Windows is supported only for host-side configuration and URL tests. The
// production c-shared library is built for Android, where AF_UNIX is available.
func setupPair(net.Conn) (int32, bool) {
	return -1, false
}
