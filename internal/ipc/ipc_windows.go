// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package ipc

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// DefaultAddr is the system-wide perflock named pipe.
const DefaultAddr = `\\.\pipe\perflock`

// Authenticated users may read and write. LocalSystem and administrators have
// full access. The pipe rejects remote clients independently of this ACL.
const pipeSecurityDescriptor = "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;AU)"

// Listen creates a local named-pipe listener. go-winio atomically creates the
// first pipe instance, so a second daemon cannot replace a live one.
func Listen(addr string) (net.Listener, error) {
	listener, err := winio.ListenPipe(addr, &winio.PipeConfig{
		SecurityDescriptor: pipeSecurityDescriptor,
		InputBufferSize:    4096,
		OutputBufferSize:   4096,
	})
	if err != nil {
		if errors.Is(err, os.ErrExist) || errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return nil, fmt.Errorf("%w: %s", ErrAlreadyRunning, addr)
		}
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	return listener, nil
}

// Dial connects to a perflock daemon.
func Dial(addr string) (net.Conn, error) {
	timeout := 2 * time.Second
	return winio.DialPipe(addr, &timeout)
}

// PeerUser returns the operating-system username of the connected peer.
func PeerUser(conn net.Conn) (string, bool) {
	file, ok := conn.(interface{ Fd() uintptr })
	if !ok {
		return "", false
	}

	var processID uint32
	if err := windows.GetNamedPipeClientProcessId(windows.Handle(file.Fd()), &processID); err != nil {
		return "", false
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, processID)
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(process)

	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &token); err != nil {
		return "", false
	}
	defer token.Close()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return "", false
	}
	account, domain, _, err := tokenUser.User.Sid.LookupAccount("")
	if err != nil {
		return "", false
	}
	if domain == "" {
		return account, true
	}
	return domain + `\` + account, true
}
