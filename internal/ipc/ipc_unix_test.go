// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows

package ipc

import (
	"errors"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testSocketAddress(t *testing.T) string {
	t.Helper()
	base := ""
	if runtime.GOOS == "darwin" {
		base = "/tmp"
	}
	dir, err := os.MkdirTemp(base, "perflock-ipc-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "perflock.socket")
}

func TestListenRefusesLiveEndpoint(t *testing.T) {
	addr := testSocketAddress(t)
	first, err := Listen(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	second, err := Listen(addr)
	if second != nil {
		second.Close()
		t.Fatal("second listener succeeded")
	}
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second listener error = %v, want %v", err, ErrAlreadyRunning)
	}

	conn, err := Dial(addr)
	if err != nil {
		t.Fatalf("first listener stopped serving: %v", err)
	}
	conn.Close()
}

func TestListenReplacesStaleEndpoint(t *testing.T) {
	addr := testSocketAddress(t)
	unixAddr, err := net.ResolveUnixAddr("unix", addr)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := net.ListenUnix("unix", unixAddr)
	if err != nil {
		t.Fatal(err)
	}
	stale.SetUnlinkOnClose(false)
	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}

	listener, err := Listen(addr)
	if err != nil {
		t.Fatalf("replace stale endpoint: %v", err)
	}
	listener.Close()
}

func TestListenPreservesNonSocketEndpoint(t *testing.T) {
	addr := testSocketAddress(t)
	const contents = "do not remove"
	if err := os.WriteFile(addr, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	listener, err := Listen(addr)
	if listener != nil {
		listener.Close()
		t.Fatal("listener replaced a regular file")
	}
	if err == nil {
		t.Fatal("listener returned no error for a regular file")
	}
	got, readErr := os.ReadFile(addr)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != contents {
		t.Fatalf("endpoint contents = %q, want %q", got, contents)
	}
}

func TestValidateUnixAddress(t *testing.T) {
	max := 0
	switch runtime.GOOS {
	case "darwin":
		max = 103
	case "linux":
		max = 107
	default:
		t.Skip("platform limit is not defined")
	}
	if err := validateUnixAddress(strings.Repeat("x", max-1)); err != nil {
		t.Fatalf("path below limit failed: %v", err)
	}
	if err := validateUnixAddress(strings.Repeat("x", max)); err != nil {
		t.Fatalf("path at limit failed: %v", err)
	}
	if err := validateUnixAddress(strings.Repeat("x", max+1)); !errors.Is(err, ErrPathTooLong) {
		t.Fatalf("path over limit error = %v, want %v", err, ErrPathTooLong)
	}
}

func TestPeerUser(t *testing.T) {
	addr := testSocketAddress(t)
	listener, err := Listen(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	result := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			result <- "accept: " + err.Error()
			return
		}
		defer conn.Close()
		name, ok := PeerUser(conn)
		if !ok {
			result <- "peer user unavailable"
			return
		}
		result <- name
	}()

	conn, err := Dial(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	if got := <-result; got != current.Username {
		t.Fatalf("peer user = %q, want %q", got, current.Username)
	}
}
