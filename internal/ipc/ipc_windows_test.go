// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package ipc

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"
	"testing"
	"time"
)

func windowsTestAddress(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf(`\\.\pipe\perflock-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
}

func TestListenRefusesLivePipe(t *testing.T) {
	addr := windowsTestAddress(t)
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

	accepted := make(chan error, 1)
	go func() {
		conn, err := first.Accept()
		if err == nil {
			err = conn.Close()
		}
		accepted <- err
	}()
	conn, err := Dial(addr)
	if err != nil {
		t.Fatalf("first listener stopped serving: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-accepted; err != nil {
		t.Fatalf("first listener failed to accept: %v", err)
	}
}

func TestPeerUserWindows(t *testing.T) {
	addr := windowsTestAddress(t)
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
	got := <-result
	if !sameWindowsAccount(got, current.Username) {
		t.Fatalf("peer user = %q, want %q", got, current.Username)
	}
}

func sameWindowsAccount(left, right string) bool {
	account := func(value string) string {
		if index := strings.LastIndexAny(value, `\\/`); index >= 0 {
			return value[index+1:]
		}
		return value
	}
	return strings.EqualFold(account(left), account(right))
}
