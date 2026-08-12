// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows

package ipc

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"runtime"
	"strings"
	"syscall"
	"time"

	"inet.af/peercred"
)

// DefaultAddr is the system-wide perflock socket path.
const DefaultAddr = "/var/run/perflock.socket"

// Listen creates a local listener. It refuses to replace a live daemon's
// socket and removes only endpoints proven stale by a refused connection.
func Listen(addr string) (net.Listener, error) {
	if err := validateUnixAddress(addr); err != nil {
		return nil, err
	}

	abstract := runtime.GOOS == "linux" && strings.HasPrefix(addr, "@")
	if !abstract {
		if err := prepareUnixAddress(addr); err != nil {
			return nil, err
		}
	}

	listener, err := net.Listen("unix", addr)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return nil, fmt.Errorf("%w: %s", ErrAlreadyRunning, addr)
		}
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	if !abstract {
		if err := os.Chmod(addr, 0777); err != nil {
			listener.Close()
			return nil, fmt.Errorf("make socket accessible: %w", err)
		}
	}
	return listener, nil
}

// Dial connects to a perflock daemon.
func Dial(addr string) (net.Conn, error) {
	if err := validateUnixAddress(addr); err != nil {
		return nil, err
	}
	return net.Dial("unix", addr)
}

// PeerUser returns the operating-system username of the connected peer.
func PeerUser(conn net.Conn) (string, bool) {
	credentials, err := peercred.Get(conn)
	if err != nil {
		return "", false
	}
	uid, ok := credentials.UserID()
	if !ok {
		return "", false
	}
	peer, err := user.LookupId(uid)
	if err != nil {
		return "", false
	}
	return peer.Username, true
}

func prepareUnixAddress(addr string) error {
	info, err := os.Lstat(addr)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect socket path: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to remove non-socket endpoint %s", addr)
	}

	conn, dialErr := net.DialTimeout("unix", addr, 200*time.Millisecond)
	if dialErr == nil {
		conn.Close()
		return fmt.Errorf("%w: %s", ErrAlreadyRunning, addr)
	}
	if !errors.Is(dialErr, syscall.ECONNREFUSED) {
		return fmt.Errorf("check existing socket %s: %w", addr, dialErr)
	}
	if err := os.Remove(addr); err != nil {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	return nil
}

func validateUnixAddress(addr string) error {
	max := 0
	switch runtime.GOOS {
	case "darwin":
		max = 103
	case "linux":
		max = 107
	}
	if max != 0 && len(addr) > max {
		return fmt.Errorf("%w: %d bytes exceeds the %d-byte limit: %q", ErrPathTooLong, len(addr), max, addr)
	}
	return nil
}
