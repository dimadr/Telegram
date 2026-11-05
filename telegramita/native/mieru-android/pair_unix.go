//go:build android || linux || darwin

package main

import (
	"io"
	"net"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// setupPair creates an AF_UNIX socketpair, hands one end to Go (net.Conn) and
// returns the other end for C++ tgnet. net.FileConn duplicates goFD, so closing
// the os.File transfers ownership cleanly to pairConn.
func setupPair(conn net.Conn) (int32, bool) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return -1, false
	}
	goFD, cppFD := fds[0], fds[1]
	f := os.NewFile(uintptr(goFD), "mieru-pair")
	pairConn, err := net.FileConn(f)
	_ = f.Close()
	if err != nil {
		_ = unix.Close(cppFD)
		return -1, false
	}
	go pump(pairConn, conn)
	return int32(cppFD), true
}

func pump(pair net.Conn, conn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(pair, conn)
		_ = pair.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(conn, pair)
		_ = conn.Close()
	}()
	wg.Wait()
}
